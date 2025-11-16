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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/weaviate/weaviate/entities/streaming"
)

// WebhookAction represents a webhook trigger action
type WebhookAction struct {
	URL     string
	Method  string
	Headers map[string]string
	Timeout time.Duration
	client  *http.Client
}

// NewWebhookAction creates a new webhook action
func NewWebhookAction(url, method string, headers map[string]string, timeout time.Duration) *WebhookAction {
	if method == "" {
		method = "POST"
	}
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	return &WebhookAction{
		URL:     url,
		Method:  method,
		Headers: headers,
		Timeout: timeout,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// Execute executes the webhook action
func (a *WebhookAction) Execute(ctx context.Context, event *streaming.ChangeEvent) error {
	// Marshal event to JSON
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, a.Method, a.URL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	for k, v := range a.Headers {
		req.Header.Set(k, v)
	}

	// Execute request
	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute webhook: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("webhook failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}
