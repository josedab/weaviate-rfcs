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

package streaming

import "time"

// KafkaConfig represents the configuration for Kafka integration
type KafkaConfig struct {
	Brokers       []string
	Topics        []string
	GroupID       string
	BatchSize     int
	FlushInterval time.Duration
	ClassMapping  map[string]ClassConfig
}

// CDCConfig represents the configuration for CDC publishing
type CDCConfig struct {
	Enabled    bool
	Topic      string
	Format     string // json | avro | protobuf | debezium
	Classes    []string
	Operations []string
}

// SubscriptionConfig represents the configuration for subscriptions
type SubscriptionConfig struct {
	Enabled        bool
	MaxSubscribers int
	BufferSize     int
}

// StreamingConfig represents the overall streaming configuration
type StreamingConfig struct {
	Kafka         *KafkaStreamConfig
	CDC           *CDCConfig
	Subscriptions *SubscriptionConfig
}

// KafkaStreamConfig represents the Kafka streaming configuration
type KafkaStreamConfig struct {
	Enabled   bool
	Brokers   []string
	Consumers []ConsumerConfig
}

// ConsumerConfig represents a single Kafka consumer configuration
type ConsumerConfig struct {
	Topics          []string
	GroupID         string
	Class           string
	Mapping         MappingConfig
	BatchSize       int
	FlushInterval   time.Duration
}

// MappingConfig represents field mapping configuration
type MappingConfig struct {
	KeyField        string
	VectorizeFields []string
}
