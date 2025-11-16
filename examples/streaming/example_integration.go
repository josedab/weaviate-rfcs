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

package main

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/weaviate/weaviate/adapters/streaming/cdc"
	"github.com/weaviate/weaviate/adapters/streaming/kafka"
	"github.com/weaviate/weaviate/entities/streaming"
	streamingUC "github.com/weaviate/weaviate/usecases/streaming"
)

// Example integration showing how to use the streaming components

// MockVectorizer is a mock implementation of the Vectorizer interface
type MockVectorizer struct{}

func (m *MockVectorizer) Vectorize(ctx context.Context, text string) ([]float32, error) {
	// Mock implementation - returns a random vector
	return []float32{0.1, 0.2, 0.3, 0.4}, nil
}

// MockBatchWriter is a mock implementation of the BatchWriter interface
type MockBatchWriter struct {
	logger *logrus.Logger
}

func (m *MockBatchWriter) WriteBatch(ctx context.Context, objects []map[string]interface{}) error {
	m.logger.Infof("Writing batch of %d objects", len(objects))
	for _, obj := range objects {
		m.logger.Debugf("Object: %+v", obj)
	}
	return nil
}

func main() {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	ctx := context.Background()

	// Example 1: Kafka Consumer for real-time ingestion
	fmt.Println("=== Example 1: Kafka Consumer ===")
	runKafkaConsumerExample(ctx, logger)

	// Example 2: CDC Publisher
	fmt.Println("\n=== Example 2: CDC Publisher ===")
	runCDCPublisherExample(ctx, logger)

	// Example 3: GraphQL Subscriptions
	fmt.Println("\n=== Example 3: GraphQL Subscriptions ===")
	runSubscriptionExample(ctx, logger)

	// Example 4: Event-Driven Triggers
	fmt.Println("\n=== Example 4: Event-Driven Triggers ===")
	runTriggerExample(ctx, logger)

	// Example 5: Full Integration
	fmt.Println("\n=== Example 5: Full Integration ===")
	runFullIntegrationExample(ctx, logger)
}

func runKafkaConsumerExample(ctx context.Context, logger *logrus.Logger) {
	// Configure Kafka consumer
	config := &streaming.KafkaConfig{
		Brokers:       []string{"localhost:9092"},
		Topics:        []string{"products"},
		GroupID:       "weaviate-products",
		BatchSize:     100,
		FlushInterval: 1 * time.Second,
		ClassMapping: map[string]streaming.ClassConfig{
			"products": {
				ClassName:       "Product",
				KeyField:        "product_id",
				VectorizeFields: []string{"name", "description"},
			},
		},
	}

	vectorizer := &MockVectorizer{}
	writer := &MockBatchWriter{logger: logger}

	consumer, err := kafka.NewConsumer(config, vectorizer, writer, logger)
	if err != nil {
		logger.Errorf("Failed to create Kafka consumer: %v", err)
		return
	}
	defer consumer.Close()

	// Start consuming (in production, this would run in a goroutine)
	logger.Info("Kafka consumer configured successfully")
	// ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	// defer cancel()
	// consumer.Start(ctx)
}

func runCDCPublisherExample(ctx context.Context, logger *logrus.Logger) {
	// Configure CDC publisher
	config := &streaming.CDCConfig{
		Enabled:    true,
		Topic:      "weaviate.changes",
		Format:     "debezium",
		Classes:    []string{"Product", "Article"},
		Operations: []string{"INSERT", "UPDATE", "DELETE"},
	}

	brokers := []string{"localhost:9092"}
	publisher, err := cdc.NewPublisher(brokers, config, logger)
	if err != nil {
		logger.Errorf("Failed to create CDC publisher: %v", err)
		return
	}
	defer publisher.Close()

	// Simulate a write event
	event := &streaming.ChangeEvent{
		Type:      streaming.ChangeTypeInsert,
		Class:     "Product",
		ID:        uuid.New(),
		After:     map[string]interface{}{"name": "Product A", "price": 99.99},
		Timestamp: time.Now(),
	}

	if err := publisher.OnWrite(ctx, event); err != nil {
		logger.Errorf("Failed to publish event: %v", err)
		return
	}

	logger.Info("CDC event published successfully")
	publisher.Flush(1000)
}

func runSubscriptionExample(ctx context.Context, logger *logrus.Logger) {
	// Configure subscription manager
	config := &streaming.SubscriptionConfig{
		Enabled:        true,
		MaxSubscribers: 1000,
		BufferSize:     100,
	}

	manager := streamingUC.NewSubscriptionManager(config, logger)
	defer manager.Close()

	// Create a subscription
	filter := &streaming.Filter{
		Class:      "Product",
		Operations: []streaming.ChangeType{streaming.ChangeTypeInsert, streaming.ChangeTypeUpdate},
	}

	subCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	channel, subID, err := manager.Subscribe(subCtx, filter)
	if err != nil {
		logger.Errorf("Failed to create subscription: %v", err)
		return
	}

	logger.Infof("Subscription created: %s", subID)

	// Publish an event
	event := &streaming.ChangeEvent{
		Type:      streaming.ChangeTypeInsert,
		Class:     "Product",
		ID:        uuid.New(),
		After:     map[string]interface{}{"name": "Product B", "price": 149.99},
		Timestamp: time.Now(),
	}

	manager.Publish(event)

	// Receive the event
	select {
	case receivedEvent := <-channel:
		logger.Infof("Received event: %s:%s (type=%s)", receivedEvent.Class, receivedEvent.ID, receivedEvent.Type)
	case <-time.After(1 * time.Second):
		logger.Warn("No event received")
	}

	// Print stats
	stats := manager.GetStats()
	logger.Infof("Subscription stats: %+v", stats)
}

func runTriggerExample(ctx context.Context, logger *logrus.Logger) {
	// Create trigger engine
	engine := streamingUC.NewTriggerEngine(4, logger)
	defer engine.Close()

	// Create a webhook trigger
	webhookAction := streamingUC.NewWebhookAction(
		"http://localhost:8080/webhook",
		"POST",
		map[string]string{"Authorization": "Bearer token123"},
		10*time.Second,
	)

	trigger := &streamingUC.Trigger{
		Name:        "product-webhook",
		Description: "Trigger webhook on product changes",
		Class:       "Product",
		Event:       streaming.EventTypeInsert,
		Filter:      &streaming.Filter{Class: "Product"},
		Action:      webhookAction,
	}

	if err := engine.RegisterTrigger(trigger); err != nil {
		logger.Errorf("Failed to register trigger: %v", err)
		return
	}

	logger.Info("Trigger registered successfully")

	// Simulate an event
	event := &streaming.ChangeEvent{
		Type:      streaming.ChangeTypeInsert,
		Class:     "Product",
		ID:        uuid.New(),
		After:     map[string]interface{}{"name": "Product C", "price": 199.99},
		Timestamp: time.Now(),
	}

	engine.OnEvent(ctx, event)
	logger.Info("Event processed by trigger engine")

	// Wait a bit for trigger execution
	time.Sleep(500 * time.Millisecond)
}

func runFullIntegrationExample(ctx context.Context, logger *logrus.Logger) {
	logger.Info("Setting up full streaming integration...")

	// 1. Setup CDC Publisher
	cdcConfig := &streaming.CDCConfig{
		Enabled:    true,
		Topic:      "weaviate.changes",
		Format:     "debezium",
		Classes:    []string{"Product"},
		Operations: []string{"INSERT", "UPDATE", "DELETE"},
	}

	cdcPublisher, err := cdc.NewPublisher([]string{"localhost:9092"}, cdcConfig, logger)
	if err != nil {
		logger.Errorf("Failed to create CDC publisher: %v", err)
		return
	}
	defer cdcPublisher.Close()

	// 2. Setup Subscription Manager
	subConfig := &streaming.SubscriptionConfig{
		Enabled:        true,
		MaxSubscribers: 1000,
		BufferSize:     100,
	}

	subManager := streamingUC.NewSubscriptionManager(subConfig, logger)
	defer subManager.Close()

	// 3. Setup Trigger Engine
	triggerEngine := streamingUC.NewTriggerEngine(4, logger)
	defer triggerEngine.Close()

	// Register a trigger
	webhookAction := streamingUC.NewWebhookAction(
		"http://localhost:8080/webhook",
		"POST",
		nil,
		10*time.Second,
	)

	trigger := &streamingUC.Trigger{
		Name:   "product-changes",
		Class:  "Product",
		Event:  streaming.EventTypeInsert,
		Action: webhookAction,
	}

	triggerEngine.RegisterTrigger(trigger)

	// 4. Create a subscription
	filter := &streaming.Filter{
		Class: "Product",
	}

	subCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	channel, _, err := subManager.Subscribe(subCtx, filter)
	if err != nil {
		logger.Errorf("Failed to create subscription: %v", err)
		return
	}

	// 5. Simulate a write operation
	event := &streaming.ChangeEvent{
		Type:  streaming.ChangeTypeInsert,
		Class: "Product",
		ID:    uuid.New(),
		After: map[string]interface{}{
			"name":        "Integrated Product",
			"price":       299.99,
			"category":    "Electronics",
			"description": "A fully integrated streaming example",
		},
		Timestamp: time.Now(),
	}

	// Publish to CDC
	if err := cdcPublisher.OnWrite(ctx, event); err != nil {
		logger.Errorf("Failed to publish CDC event: %v", err)
	} else {
		logger.Info("CDC event published")
	}

	// Publish to subscribers
	subManager.Publish(event)
	logger.Info("Event published to subscribers")

	// Process triggers
	triggerEngine.OnEvent(ctx, event)
	logger.Info("Event sent to trigger engine")

	// Receive via subscription
	select {
	case receivedEvent := <-channel:
		logger.Infof("Subscription received: %s:%s", receivedEvent.Class, receivedEvent.ID)
	case <-time.After(1 * time.Second):
		logger.Warn("No subscription event received")
	}

	logger.Info("Full integration example completed successfully!")
}
