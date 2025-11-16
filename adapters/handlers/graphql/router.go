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

package graphql

import (
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
	v2 "github.com/weaviate/weaviate/adapters/handlers/graphql/v2"
	"github.com/weaviate/weaviate/adapters/handlers/graphql/v2/resolvers"
	"github.com/weaviate/weaviate/entities/schema"
)

// VersionedRouterConfig holds configuration for versioned routing
type VersionedRouterConfig struct {
	V1Enabled         bool
	V2Enabled         bool
	V2Default         bool
	V1Path            string
	V2Path            string
	V1DeprecationDate string
	MaxComplexity     int
	Logger            logrus.FieldLogger
}

// DefaultVersionedRouterConfig returns default configuration
func DefaultVersionedRouterConfig() VersionedRouterConfig {
	return VersionedRouterConfig{
		V1Enabled:         true,
		V2Enabled:         true,
		V2Default:         false, // Keep v1 as default for backward compatibility
		V1Path:            "/v1/graphql",
		V2Path:            "/v2/graphql",
		V1DeprecationDate: "2026-01-01",
		MaxComplexity:     10000,
	}
}

// VersionedRouter routes requests to appropriate GraphQL API version
type VersionedRouter struct {
	v1Handler http.Handler
	v2Handler http.Handler
	config    VersionedRouterConfig
	logger    logrus.FieldLogger
}

// NewVersionedRouter creates a new versioned router
func NewVersionedRouter(
	weaviateSchema *schema.Schema,
	v1Traverser Traverser,
	v2Repository resolvers.Repository,
	config VersionedRouterConfig,
) (*VersionedRouter, error) {
	if config.Logger == nil {
		config.Logger = logrus.New()
	}

	router := &VersionedRouter{
		config: config,
		logger: config.Logger,
	}

	// Set up v1 handler if enabled
	if config.V1Enabled {
		// Use existing v1 GraphQL handler
		// This would wrap the existing graphQL struct
		router.v1Handler = &deprecationWarningHandler{
			deprecationDate: config.V1DeprecationDate,
			logger:          config.Logger,
			// Actual v1 handler would be passed here
		}
	}

	// Set up v2 handler if enabled
	if config.V2Enabled {
		v2Config := v2.Config{
			MaxComplexity:  config.MaxComplexity,
			EnableV1Compat: config.V1Enabled, // Enable translation if v1 is still enabled
			Logger:         config.Logger,
		}

		v2h, err := v2.NewHandler(weaviateSchema, v2Repository, v2Config)
		if err != nil {
			return nil, err
		}

		router.v2Handler = v2h
	}

	return router, nil
}

// ServeHTTP routes requests based on path
func (r *VersionedRouter) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// Route based on path
	switch req.URL.Path {
	case r.config.V1Path:
		if r.v1Handler != nil {
			r.v1Handler.ServeHTTP(w, req)
		} else {
			r.writeError(w, http.StatusNotFound, "GraphQL v1 API is not enabled")
		}

	case r.config.V2Path:
		if r.v2Handler != nil {
			r.v2Handler.ServeHTTP(w, req)
		} else {
			r.writeError(w, http.StatusNotFound, "GraphQL v2 API is not enabled")
		}

	default:
		// Default version
		if r.config.V2Default && r.v2Handler != nil {
			r.v2Handler.ServeHTTP(w, req)
		} else if r.v1Handler != nil {
			r.v1Handler.ServeHTTP(w, req)
		} else {
			r.writeError(w, http.StatusNotFound, "No GraphQL API enabled")
		}
	}
}

// writeError writes an error response
func (r *VersionedRouter) writeError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	w.Write([]byte(`{"errors":[{"message":"` + message + `"}]}`))
}

// deprecationWarningHandler wraps v1 handler with deprecation warnings
type deprecationWarningHandler struct {
	handler         http.Handler
	deprecationDate string
	logger          logrus.FieldLogger
}

func (h *deprecationWarningHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Add deprecation warning header
	w.Header().Set("X-API-Deprecated", "true")
	w.Header().Set("X-API-Deprecation-Date", h.deprecationDate)
	w.Header().Set("X-API-Sunset", h.deprecationDate)
	w.Header().Set("Link", `</v2/graphql>; rel="alternate"`)

	// Log deprecation warning
	h.logger.WithFields(logrus.Fields{
		"path":             r.URL.Path,
		"deprecation_date": h.deprecationDate,
		"client_ip":        r.RemoteAddr,
		"user_agent":       r.UserAgent(),
	}).Warn("GraphQL v1 API is deprecated and will be removed")

	// Call wrapped handler
	if h.handler != nil {
		h.handler.ServeHTTP(w, r)
	}
}

// VersionInfo holds API version information
type VersionInfo struct {
	Version           string    `json:"version"`
	Enabled           bool      `json:"enabled"`
	Deprecated        bool      `json:"deprecated,omitempty"`
	DeprecationDate   string    `json:"deprecationDate,omitempty"`
	Path              string    `json:"path"`
}

// GetVersionInfo returns information about available API versions
func (r *VersionedRouter) GetVersionInfo() []VersionInfo {
	info := []VersionInfo{}

	if r.config.V1Enabled {
		info = append(info, VersionInfo{
			Version:         "v1",
			Enabled:         true,
			Deprecated:      true,
			DeprecationDate: r.config.V1DeprecationDate,
			Path:            r.config.V1Path,
		})
	}

	if r.config.V2Enabled {
		info = append(info, VersionInfo{
			Version:    "v2",
			Enabled:    true,
			Deprecated: false,
			Path:       r.config.V2Path,
		})
	}

	return info
}

// Middleware for automatic version detection and routing
func (r *VersionedRouter) VersionDetectionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		// Check for version header
		apiVersion := req.Header.Get("X-API-Version")

		// Route based on version header
		switch apiVersion {
		case "1":
			if r.v1Handler != nil {
				r.v1Handler.ServeHTTP(w, req)
				return
			}
		case "2":
			if r.v2Handler != nil {
				r.v2Handler.ServeHTTP(w, req)
				return
			}
		}

		// Default routing
		next.ServeHTTP(w, req)
	})
}

// Middleware for rate limiting
type RateLimiter struct {
	requests map[string][]time.Time
	limit    int
	window   time.Duration
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		requests: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientID := r.RemoteAddr // In production, use API key or user ID

		now := time.Now()
		cutoff := now.Add(-rl.window)

		// Clean old requests
		requests := rl.requests[clientID]
		newRequests := []time.Time{}
		for _, t := range requests {
			if t.After(cutoff) {
				newRequests = append(newRequests, t)
			}
		}

		// Check limit
		if len(newRequests) >= rl.limit {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-RateLimit-Limit", string(rl.limit))
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"errors":[{"message":"Rate limit exceeded","extensions":{"code":"RATE_LIMIT_EXCEEDED"}}]}`))
			return
		}

		// Add current request
		newRequests = append(newRequests, now)
		rl.requests[clientID] = newRequests

		// Set rate limit headers
		w.Header().Set("X-RateLimit-Limit", string(rl.limit))
		w.Header().Set("X-RateLimit-Remaining", string(rl.limit-len(newRequests)))

		next.ServeHTTP(w, r)
	})
}
