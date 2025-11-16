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

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/weaviate/weaviate/entities/streaming"
)

// Subscriber represents a subscription to change events
type Subscriber struct {
	ID      string
	Filter  *streaming.Filter
	Channel chan *streaming.ChangeEvent
	created time.Time
}

// SubscriptionManager manages subscriptions to change events
type SubscriptionManager struct {
	subscribers    map[string][]*Subscriber // class -> subscribers
	subscriberByID map[string]*Subscriber   // id -> subscriber
	mu             sync.RWMutex
	config         *streaming.SubscriptionConfig
	logger         *logrus.Logger
}

// NewSubscriptionManager creates a new subscription manager
func NewSubscriptionManager(
	config *streaming.SubscriptionConfig,
	logger *logrus.Logger,
) *SubscriptionManager {
	if config == nil {
		config = &streaming.SubscriptionConfig{
			Enabled:        true,
			MaxSubscribers: 10000,
			BufferSize:     100,
		}
	}

	return &SubscriptionManager{
		subscribers:    make(map[string][]*Subscriber),
		subscriberByID: make(map[string]*Subscriber),
		config:         config,
		logger:         logger,
	}
}

// Subscribe creates a new subscription to change events
func (m *SubscriptionManager) Subscribe(
	ctx context.Context,
	filter *streaming.Filter,
) (<-chan *streaming.ChangeEvent, string, error) {
	if !m.config.Enabled {
		return nil, "", fmt.Errorf("subscriptions are not enabled")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check max subscribers limit
	totalSubscribers := len(m.subscriberByID)
	if totalSubscribers >= m.config.MaxSubscribers {
		return nil, "", fmt.Errorf("max subscribers limit reached (%d)", m.config.MaxSubscribers)
	}

	// Create subscriber
	sub := &Subscriber{
		ID:      uuid.New().String(),
		Filter:  filter,
		Channel: make(chan *streaming.ChangeEvent, m.config.BufferSize),
		created: time.Now(),
	}

	// Add to maps
	if filter.Class != "" {
		m.subscribers[filter.Class] = append(m.subscribers[filter.Class], sub)
	} else {
		// Subscribe to all classes
		m.subscribers["*"] = append(m.subscribers["*"], sub)
	}
	m.subscriberByID[sub.ID] = sub

	m.logger.Infof("New subscription created: %s (class=%s, total=%d)", sub.ID, filter.Class, totalSubscribers+1)

	// Cleanup on context cancellation
	go func() {
		<-ctx.Done()
		m.unsubscribe(sub.ID)
	}()

	return sub.Channel, sub.ID, nil
}

// Unsubscribe removes a subscription
func (m *SubscriptionManager) Unsubscribe(id string) error {
	return m.unsubscribe(id)
}

// unsubscribe removes a subscription (internal)
func (m *SubscriptionManager) unsubscribe(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	sub, ok := m.subscriberByID[id]
	if !ok {
		return fmt.Errorf("subscription not found: %s", id)
	}

	// Remove from class subscribers
	class := sub.Filter.Class
	if class == "" {
		class = "*"
	}

	subs := m.subscribers[class]
	for i, s := range subs {
		if s.ID == id {
			m.subscribers[class] = append(subs[:i], subs[i+1:]...)
			break
		}
	}

	// Remove from ID map
	delete(m.subscriberByID, id)

	// Close channel
	close(sub.Channel)

	m.logger.Infof("Subscription removed: %s (class=%s, duration=%s)", id, class, time.Since(sub.created))
	return nil
}

// Publish publishes a change event to subscribers
func (m *SubscriptionManager) Publish(event *streaming.ChangeEvent) {
	if !m.config.Enabled {
		return
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	// Get class-specific subscribers
	classSubs := m.subscribers[event.Class]

	// Get wildcard subscribers
	wildcardSubs := m.subscribers["*"]

	// Combine both lists
	allSubs := make([]*Subscriber, 0, len(classSubs)+len(wildcardSubs))
	allSubs = append(allSubs, classSubs...)
	allSubs = append(allSubs, wildcardSubs...)

	// Publish to matching subscribers
	published := 0
	dropped := 0

	for _, sub := range allSubs {
		if sub.Filter.Matches(event) {
			select {
			case sub.Channel <- event:
				published++
			case <-time.After(100 * time.Millisecond):
				// Slow subscriber, drop event
				dropped++
				m.logger.Warnf("Dropped event for slow subscriber %s", sub.ID)
			}
		}
	}

	if published > 0 || dropped > 0 {
		m.logger.Debugf("Published event %s:%s to %d subscribers (%d dropped)",
			event.Class, event.ID, published, dropped)
	}
}

// GetStats returns subscription statistics
func (m *SubscriptionManager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	classCounts := make(map[string]int)
	for class, subs := range m.subscribers {
		classCounts[class] = len(subs)
	}

	return map[string]interface{}{
		"total_subscribers": len(m.subscriberByID),
		"by_class":          classCounts,
		"max_subscribers":   m.config.MaxSubscribers,
		"buffer_size":       m.config.BufferSize,
	}
}

// Close closes all subscriptions
func (m *SubscriptionManager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.logger.Infof("Closing subscription manager (%d active subscriptions)", len(m.subscriberByID))

	for id := range m.subscriberByID {
		sub := m.subscriberByID[id]
		close(sub.Channel)
	}

	m.subscribers = make(map[string][]*Subscriber)
	m.subscriberByID = make(map[string]*Subscriber)
}
