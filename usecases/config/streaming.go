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

package config

import (
	"os"
	"strconv"
	"strings"
	"time"

	entcfg "github.com/weaviate/weaviate/entities/config"
	"github.com/weaviate/weaviate/entities/streaming"
)

// Streaming represents the streaming configuration
type Streaming struct {
	Kafka         *KafkaStreaming         `json:"kafka" yaml:"kafka"`
	CDC           *streaming.CDCConfig    `json:"cdc" yaml:"cdc"`
	Subscriptions *SubscriptionConfigExt  `json:"subscriptions" yaml:"subscriptions"`
}

// KafkaStreaming represents the Kafka streaming configuration
type KafkaStreaming struct {
	Enabled   bool                         `json:"enabled" yaml:"enabled"`
	Brokers   []string                     `json:"brokers" yaml:"brokers"`
	Consumers []streaming.ConsumerConfig   `json:"consumers" yaml:"consumers"`
}

// SubscriptionConfigExt extends the basic subscription config
type SubscriptionConfigExt struct {
	streaming.SubscriptionConfig `yaml:",inline"`
}

// ParseStreamingConfig parses streaming configuration from environment variables
func ParseStreamingConfig() *Streaming {
	config := &Streaming{
		Kafka: &KafkaStreaming{
			Enabled:   false,
			Brokers:   []string{},
			Consumers: []streaming.ConsumerConfig{},
		},
		CDC: &streaming.CDCConfig{
			Enabled:    false,
			Topic:      "weaviate.changes",
			Format:     "debezium",
			Classes:    []string{},
			Operations: []string{"INSERT", "UPDATE", "DELETE"},
		},
		Subscriptions: &SubscriptionConfigExt{
			SubscriptionConfig: streaming.SubscriptionConfig{
				Enabled:        false,
				MaxSubscribers: 10000,
				BufferSize:     100,
			},
		},
	}

	// Kafka configuration
	if entcfg.Enabled(os.Getenv("STREAMING_KAFKA_ENABLED")) {
		config.Kafka.Enabled = true
	}

	if brokers := os.Getenv("STREAMING_KAFKA_BROKERS"); brokers != "" {
		config.Kafka.Brokers = strings.Split(brokers, ",")
	}

	// CDC configuration
	if entcfg.Enabled(os.Getenv("STREAMING_CDC_ENABLED")) {
		config.CDC.Enabled = true
	}

	if topic := os.Getenv("STREAMING_CDC_TOPIC"); topic != "" {
		config.CDC.Topic = topic
	}

	if format := os.Getenv("STREAMING_CDC_FORMAT"); format != "" {
		config.CDC.Format = format
	}

	if classes := os.Getenv("STREAMING_CDC_CLASSES"); classes != "" {
		config.CDC.Classes = strings.Split(classes, ",")
	}

	if operations := os.Getenv("STREAMING_CDC_OPERATIONS"); operations != "" {
		config.CDC.Operations = strings.Split(operations, ",")
	}

	// Subscriptions configuration
	if entcfg.Enabled(os.Getenv("STREAMING_SUBSCRIPTIONS_ENABLED")) {
		config.Subscriptions.Enabled = true
	}

	if maxSubs := os.Getenv("STREAMING_SUBSCRIPTIONS_MAX"); maxSubs != "" {
		if val, err := strconv.Atoi(maxSubs); err == nil && val > 0 {
			config.Subscriptions.MaxSubscribers = val
		}
	}

	if bufferSize := os.Getenv("STREAMING_SUBSCRIPTIONS_BUFFER_SIZE"); bufferSize != "" {
		if val, err := strconv.Atoi(bufferSize); err == nil && val > 0 {
			config.Subscriptions.BufferSize = val
		}
	}

	return config
}

// ValidateStreamingConfig validates the streaming configuration
func ValidateStreamingConfig(config *Streaming) error {
	// Validate Kafka config
	if config.Kafka != nil && config.Kafka.Enabled {
		if len(config.Kafka.Brokers) == 0 {
			return &InvalidConfigError{
				message: "Kafka streaming is enabled but no brokers are configured",
			}
		}
	}

	// Validate CDC config
	if config.CDC != nil && config.CDC.Enabled {
		if config.CDC.Topic == "" {
			return &InvalidConfigError{
				message: "CDC is enabled but no topic is configured",
			}
		}

		validFormats := map[string]bool{
			"json":     true,
			"avro":     true,
			"protobuf": true,
			"debezium": true,
		}

		if !validFormats[config.CDC.Format] {
			return &InvalidConfigError{
				message: "invalid CDC format: " + config.CDC.Format,
			}
		}
	}

	return nil
}

// InvalidConfigError represents a configuration validation error
type InvalidConfigError struct {
	message string
}

func (e *InvalidConfigError) Error() string {
	return e.message
}

// GetDefaultStreamingConfig returns the default streaming configuration
func GetDefaultStreamingConfig() *Streaming {
	return &Streaming{
		Kafka: &KafkaStreaming{
			Enabled:   false,
			Brokers:   []string{},
			Consumers: []streaming.ConsumerConfig{},
		},
		CDC: &streaming.CDCConfig{
			Enabled:    false,
			Topic:      "weaviate.changes",
			Format:     "debezium",
			Classes:    []string{},
			Operations: []string{"INSERT", "UPDATE", "DELETE"},
		},
		Subscriptions: &SubscriptionConfigExt{
			SubscriptionConfig: streaming.SubscriptionConfig{
				Enabled:        false,
				MaxSubscribers: 10000,
				BufferSize:     100,
			},
		},
	}
}

// Example configuration YAML
const StreamingConfigExample = `
# Streaming configuration
streaming:
  # Kafka streaming
  kafka:
    enabled: true
    brokers:
      - "kafka-1:9092"
      - "kafka-2:9092"
    consumers:
      - topics: ["products"]
        groupId: "weaviate-products"
        class: "Product"
        mapping:
          keyField: "product_id"
          vectorizeFields: ["name", "description"]
        batchSize: 1000
        flushInterval: "1s"

  # Change Data Capture
  cdc:
    enabled: true
    topic: "weaviate.changes"
    format: "debezium"
    classes: ["Article", "Product"]
    operations: ["INSERT", "UPDATE", "DELETE"]

  # GraphQL Subscriptions
  subscriptions:
    enabled: true
    maxSubscribers: 10000
    bufferSize: 100
`
