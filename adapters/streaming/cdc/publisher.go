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

package cdc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/sirupsen/logrus"
	"github.com/weaviate/weaviate/entities/streaming"
)

// CDCFilter is a filter for CDC events
type CDCFilter interface {
	ShouldPublish(event *streaming.ChangeEvent) bool
}

// Publisher publishes change data capture events to Kafka
type Publisher struct {
	producer *kafka.Producer
	config   *streaming.CDCConfig
	filters  []CDCFilter
	logger   *logrus.Logger
}

// NewPublisher creates a new CDC publisher
func NewPublisher(
	brokers []string,
	config *streaming.CDCConfig,
	logger *logrus.Logger,
) (*Publisher, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	if !config.Enabled {
		return &Publisher{
			config: config,
			logger: logger,
		}, nil
	}

	kafkaConfig := &kafka.ConfigMap{
		"bootstrap.servers": strings.Join(brokers, ","),
		"client.id":         "weaviate-cdc-publisher",
	}

	producer, err := kafka.NewProducer(kafkaConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kafka producer: %w", err)
	}

	// Start delivery reports handler
	go func() {
		for e := range producer.Events() {
			switch ev := e.(type) {
			case *kafka.Message:
				if ev.TopicPartition.Error != nil {
					logger.Errorf("Failed to deliver message: %v", ev.TopicPartition.Error)
				} else {
					logger.Debugf("Delivered message to %v", ev.TopicPartition)
				}
			}
		}
	}()

	return &Publisher{
		producer: producer,
		config:   config,
		filters:  make([]CDCFilter, 0),
		logger:   logger,
	}, nil
}

// AddFilter adds a filter to the publisher
func (p *Publisher) AddFilter(filter CDCFilter) {
	p.filters = append(p.filters, filter)
}

// OnWrite is called when a write operation occurs
func (p *Publisher) OnWrite(ctx context.Context, event *streaming.ChangeEvent) error {
	if !p.config.Enabled {
		return nil
	}

	// Apply filters
	if !p.shouldPublish(event) {
		return nil
	}

	// Serialize event
	data, err := p.serialize(event)
	if err != nil {
		return fmt.Errorf("failed to serialize event: %w", err)
	}

	// Publish to Kafka
	topic := p.config.Topic
	msg := &kafka.Message{
		TopicPartition: kafka.TopicPartition{
			Topic:     &topic,
			Partition: kafka.PartitionAny,
		},
		Key:   []byte(event.ID.String()),
		Value: data,
		Headers: []kafka.Header{
			{Key: "type", Value: []byte(event.Type)},
			{Key: "class", Value: []byte(event.Class)},
		},
	}

	if err := p.producer.Produce(msg, nil); err != nil {
		return fmt.Errorf("failed to produce message: %w", err)
	}

	p.logger.Debugf("Published CDC event for %s:%s (type=%s)", event.Class, event.ID, event.Type)
	return nil
}

// shouldPublish checks if an event should be published
func (p *Publisher) shouldPublish(event *streaming.ChangeEvent) bool {
	// Check class filter
	if len(p.config.Classes) > 0 {
		found := false
		for _, class := range p.config.Classes {
			if class == event.Class {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check operation filter
	if len(p.config.Operations) > 0 {
		found := false
		for _, op := range p.config.Operations {
			if op == string(event.Type) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Apply custom filters
	for _, filter := range p.filters {
		if !filter.ShouldPublish(event) {
			return false
		}
	}

	return true
}

// serialize serializes the event based on the configured format
func (p *Publisher) serialize(event *streaming.ChangeEvent) ([]byte, error) {
	switch p.config.Format {
	case "debezium":
		return p.serializeDebezium(event)
	case "json":
		return json.Marshal(event)
	default:
		return json.Marshal(event)
	}
}

// serializeDebezium serializes the event in Debezium format
func (p *Publisher) serializeDebezium(event *streaming.ChangeEvent) ([]byte, error) {
	op := ""
	switch event.Type {
	case streaming.ChangeTypeInsert:
		op = "c" // create
	case streaming.ChangeTypeUpdate:
		op = "u" // update
	case streaming.ChangeTypeDelete:
		op = "d" // delete
	case streaming.ChangeTypeRead:
		op = "r" // read
	}

	debeziumEvent := streaming.DebeziumEvent{
		Before: event.Before,
		After:  event.After,
		Source: streaming.SourceMetadata{
			Version:   "1.0.0",
			Connector: "weaviate",
			Name:      "weaviate-cdc",
			TsMs:      event.Timestamp.UnixMilli(),
			DB:        "weaviate",
			Table:     event.Class,
		},
		Op:   op,
		TsMs: event.Timestamp.UnixMilli(),
	}

	return json.Marshal(debeziumEvent)
}

// Flush flushes any pending messages
func (p *Publisher) Flush(timeout int) int {
	if p.producer == nil {
		return 0
	}
	return p.producer.Flush(timeout)
}

// Close closes the publisher
func (p *Publisher) Close() {
	if p.producer == nil {
		return
	}
	p.logger.Info("Closing CDC publisher")
	p.producer.Flush(5000)
	p.producer.Close()
}
