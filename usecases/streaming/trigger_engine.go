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

	"github.com/sirupsen/logrus"
	"github.com/weaviate/weaviate/entities/streaming"
)

// TriggerAction represents an action to execute when a trigger fires
type TriggerAction interface {
	Execute(ctx context.Context, event *streaming.ChangeEvent) error
}

// Trigger represents an event-driven trigger
type Trigger struct {
	Name        string
	Description string
	Class       string
	Event       streaming.EventType
	Filter      *streaming.Filter
	Action      TriggerAction
	Config      map[string]interface{}
}

// TriggerEngine executes triggers based on change events
type TriggerEngine struct {
	triggers map[string]*Trigger // name -> trigger
	executor *TriggerExecutor
	mu       sync.RWMutex
	logger   *logrus.Logger
}

// TriggerExecutor executes trigger actions
type TriggerExecutor struct {
	workers int
	queue   chan *triggerJob
	logger  *logrus.Logger
	wg      sync.WaitGroup
}

type triggerJob struct {
	trigger *Trigger
	event   *streaming.ChangeEvent
}

// NewTriggerEngine creates a new trigger engine
func NewTriggerEngine(workers int, logger *logrus.Logger) *TriggerEngine {
	executor := &TriggerExecutor{
		workers: workers,
		queue:   make(chan *triggerJob, 1000),
		logger:  logger,
	}

	// Start workers
	for i := 0; i < workers; i++ {
		executor.wg.Add(1)
		go executor.worker(i)
	}

	return &TriggerEngine{
		triggers: make(map[string]*Trigger),
		executor: executor,
		logger:   logger,
	}
}

// RegisterTrigger registers a new trigger
func (e *TriggerEngine) RegisterTrigger(trigger *Trigger) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.triggers[trigger.Name]; exists {
		return fmt.Errorf("trigger already exists: %s", trigger.Name)
	}

	e.triggers[trigger.Name] = trigger
	e.logger.Infof("Registered trigger: %s (class=%s, event=%s)", trigger.Name, trigger.Class, trigger.Event)
	return nil
}

// UnregisterTrigger removes a trigger
func (e *TriggerEngine) UnregisterTrigger(name string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.triggers[name]; !exists {
		return fmt.Errorf("trigger not found: %s", name)
	}

	delete(e.triggers, name)
	e.logger.Infof("Unregistered trigger: %s", name)
	return nil
}

// OnEvent processes a change event and executes matching triggers
func (e *TriggerEngine) OnEvent(ctx context.Context, event *streaming.ChangeEvent) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, trigger := range e.triggers {
		if e.shouldExecute(trigger, event) {
			// Queue trigger execution
			select {
			case e.executor.queue <- &triggerJob{
				trigger: trigger,
				event:   event,
			}:
				e.logger.Debugf("Queued trigger %s for event %s:%s", trigger.Name, event.Class, event.ID)
			default:
				e.logger.Warnf("Trigger queue full, dropping execution of trigger %s", trigger.Name)
			}
		}
	}
}

// shouldExecute checks if a trigger should execute for an event
func (e *TriggerEngine) shouldExecute(trigger *Trigger, event *streaming.ChangeEvent) bool {
	// Check class
	if trigger.Class != "" && trigger.Class != event.Class {
		return false
	}

	// Check event type
	eventType := streaming.EventType(event.Type)
	if trigger.Event != "" && trigger.Event != eventType {
		return false
	}

	// Check filter
	if trigger.Filter != nil && !trigger.Filter.Matches(event) {
		return false
	}

	return true
}

// GetTriggers returns all registered triggers
func (e *TriggerEngine) GetTriggers() []*Trigger {
	e.mu.RLock()
	defer e.mu.RUnlock()

	triggers := make([]*Trigger, 0, len(e.triggers))
	for _, trigger := range e.triggers {
		triggers = append(triggers, trigger)
	}
	return triggers
}

// Close closes the trigger engine
func (e *TriggerEngine) Close() {
	e.logger.Info("Closing trigger engine")
	close(e.executor.queue)
	e.executor.wg.Wait()
}

// worker processes trigger jobs
func (ex *TriggerExecutor) worker(id int) {
	defer ex.wg.Done()

	ex.logger.Debugf("Trigger worker %d started", id)

	for job := range ex.queue {
		ctx := context.Background()
		if err := job.trigger.Action.Execute(ctx, job.event); err != nil {
			ex.logger.Errorf("Trigger %s failed: %v", job.trigger.Name, err)
		} else {
			ex.logger.Debugf("Trigger %s executed successfully", job.trigger.Name)
		}
	}

	ex.logger.Debugf("Trigger worker %d stopped", id)
}
