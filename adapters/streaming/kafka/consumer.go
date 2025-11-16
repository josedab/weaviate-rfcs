//                           _       _
// __      _____  __ ___   ___  __ _| |_ ___
// \ \ /\ / / _ \/ _` \ \ / / |/ _` | __/ _ \
//  \ V  V /  __/ (_| |\ V /| | (_| | ||  __/
//   \_/\_/ \___|\__,_| \_/ |_|\__,_|\__\___|
//
//  Copyright © 2016 - 2025 Weaviate B.V. All rights reserved.
//
//  CONTACT: hello@weaviate.io
//

package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/weaviate/weaviate/entities/streaming"
)

// Vectorizer is the interface for vectorizing text
type Vectorizer interface {
	Vectorize(ctx context.Context, text string) ([]float32, error)
}

// BatchWriter is the interface for writing batches of objects
type BatchWriter interface {
	WriteBatch(ctx context.Context, objects []map[string]interface{}) error
}

// Consumer represents a Kafka consumer for real-time data ingestion
type Consumer struct {
	consumer   *kafka.Consumer
	vectorizer Vectorizer
	writer     BatchWriter
	config     *streaming.KafkaConfig
	logger     *logrus.Logger
}

// NewConsumer creates a new Kafka consumer
func NewConsumer(
	config *streaming.KafkaConfig,
	vectorizer Vectorizer,
	writer BatchWriter,
	logger *logrus.Logger,
) (*Consumer, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	kafkaConfig := &kafka.ConfigMap{
		"bootstrap.servers": strings.Join(config.Brokers, ","),
		"group.id":          config.GroupID,
		"auto.offset.reset": "earliest",
		"enable.auto.commit": false,
	}

	consumer, err := kafka.NewConsumer(kafkaConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kafka consumer: %w", err)
	}

	return &Consumer{
		consumer:   consumer,
		vectorizer: vectorizer,
		writer:     writer,
		config:     config,
		logger:     logger,
	}, nil
}

// Start starts consuming messages from Kafka
func (k *Consumer) Start(ctx context.Context) error {
	// Subscribe to topics
	if err := k.consumer.SubscribeTopics(k.config.Topics, nil); err != nil {
		return fmt.Errorf("failed to subscribe to topics: %w", err)
	}

	k.logger.Infof("Kafka consumer started, subscribed to topics: %v", k.config.Topics)

	batch := make([]map[string]interface{}, 0, k.config.BatchSize)
	ticker := time.NewTicker(k.config.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			k.logger.Info("Context cancelled, flushing remaining messages")
			if len(batch) > 0 {
				return k.flush(ctx, batch)
			}
			return ctx.Err()

		case <-ticker.C:
			if len(batch) > 0 {
				k.logger.Debugf("Flush interval reached, flushing %d messages", len(batch))
				if err := k.flush(ctx, batch); err != nil {
					return fmt.Errorf("failed to flush batch on timer: %w", err)
				}
				batch = batch[:0]
			}

		default:
			// Poll for messages
			msg, err := k.consumer.ReadMessage(100 * time.Millisecond)
			if err != nil {
				// Timeout is expected, continue
				if err.(kafka.Error).Code() == kafka.ErrTimedOut {
					continue
				}
				k.logger.Errorf("Error reading message: %v", err)
				continue
			}

			// Transform message to object
			obj, err := k.transform(ctx, msg)
			if err != nil {
				k.logger.Errorf("Failed to transform message: %v", err)
				continue
			}

			// Add to batch
			batch = append(batch, obj)

			// Flush if batch full
			if len(batch) >= k.config.BatchSize {
				k.logger.Debugf("Batch size reached, flushing %d messages", len(batch))
				if err := k.flush(ctx, batch); err != nil {
					return fmt.Errorf("failed to flush batch: %w", err)
				}
				batch = batch[:0]
			}

			// Commit offset
			if _, err := k.consumer.CommitMessage(msg); err != nil {
				k.logger.Errorf("Failed to commit message: %v", err)
			}
		}
	}
}

// transform transforms a Kafka message to an object
func (k *Consumer) transform(ctx context.Context, msg *kafka.Message) (map[string]interface{}, error) {
	// Parse JSON
	var data map[string]interface{}
	if err := json.Unmarshal(msg.Value, &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal message: %w", err)
	}

	// Get class config
	topic := *msg.TopicPartition.Topic
	classConfig, ok := k.config.ClassMapping[topic]
	if !ok {
		return nil, fmt.Errorf("no class mapping found for topic %s", topic)
	}

	// Extract ID
	idValue, ok := data[classConfig.KeyField]
	if !ok {
		return nil, fmt.Errorf("key field %s not found in message", classConfig.KeyField)
	}

	// Parse or generate UUID
	var id uuid.UUID
	switch v := idValue.(type) {
	case string:
		var err error
		id, err = uuid.Parse(v)
		if err != nil {
			// If not a valid UUID, generate one
			id = uuid.New()
		}
	default:
		id = uuid.New()
	}

	// Vectorize if needed
	var vector []float32
	if k.vectorizer != nil && len(classConfig.VectorizeFields) > 0 {
		text := k.extractText(data, classConfig.VectorizeFields)
		var err error
		vector, err = k.vectorizer.Vectorize(ctx, text)
		if err != nil {
			return nil, fmt.Errorf("failed to vectorize: %w", err)
		}
	}

	// Create object
	obj := map[string]interface{}{
		"id":         id.String(),
		"class":      classConfig.ClassName,
		"properties": data,
	}

	if vector != nil {
		obj["vector"] = vector
	}

	// Apply custom transformation if provided
	if classConfig.TransformFunc != nil {
		return classConfig.TransformFunc(data)
	}

	return obj, nil
}

// extractText extracts and concatenates text from specified fields
func (k *Consumer) extractText(data map[string]interface{}, fields []string) string {
	var parts []string
	for _, field := range fields {
		if value, ok := data[field]; ok {
			if str, ok := value.(string); ok {
				parts = append(parts, str)
			}
		}
	}
	return strings.Join(parts, " ")
}

// flush writes the batch to storage
func (k *Consumer) flush(ctx context.Context, batch []map[string]interface{}) error {
	if len(batch) == 0 {
		return nil
	}

	k.logger.Infof("Flushing batch of %d objects", len(batch))

	if err := k.writer.WriteBatch(ctx, batch); err != nil {
		return fmt.Errorf("failed to write batch: %w", err)
	}

	return nil
}

// Close closes the Kafka consumer
func (k *Consumer) Close() error {
	k.logger.Info("Closing Kafka consumer")
	return k.consumer.Close()
}
