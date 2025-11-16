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

package monitoring

import (
	"context"
)

// TracingConfig holds OpenTelemetry tracing configuration
type TracingConfig struct {
	Enabled       bool
	ServiceName   string
	OTLPEndpoint  string
	SamplingRatio float64 // 0.0 to 1.0, e.g., 0.1 for 10% sampling
}

// Tracer provides distributed tracing capabilities
type Tracer struct {
	enabled bool
	// In a real implementation, this would contain OpenTelemetry tracer
	// tracer trace.Tracer
}

// NewTracer creates a new tracer instance
func NewTracer(cfg TracingConfig) (*Tracer, error) {
	if !cfg.Enabled {
		return &Tracer{enabled: false}, nil
	}

	// TODO: Initialize OpenTelemetry tracer
	// This would include:
	// 1. Creating OTLP exporter
	// 2. Setting up trace provider
	// 3. Configuring sampling based on cfg.SamplingRatio
	// 4. Registering global tracer

	return &Tracer{
		enabled: true,
	}, nil
}

// StartSpan starts a new tracing span
// In a real implementation, this would use OpenTelemetry's trace.Tracer
func (t *Tracer) StartSpan(ctx context.Context, name string, attributes ...Attribute) (context.Context, SpanFinisher) {
	if !t.enabled {
		return ctx, noOpSpanFinisher
	}

	// TODO: Implement with OpenTelemetry
	// ctx, span := t.tracer.Start(ctx, name)
	// for _, attr := range attributes {
	//     span.SetAttributes(attr.toOTel())
	// }
	// return ctx, &spanFinisher{span: span}

	return ctx, noOpSpanFinisher
}

// Attribute represents a tracing attribute
type Attribute struct {
	Key   string
	Value interface{}
}

// SpanFinisher is an interface for finishing spans
type SpanFinisher interface {
	End()
	SetAttributes(...Attribute)
	RecordError(error)
}

type noOpSpanFinisherType struct{}

func (n noOpSpanFinisherType) End()                        {}
func (n noOpSpanFinisherType) SetAttributes(...Attribute)  {}
func (n noOpSpanFinisherType) RecordError(error)           {}

var noOpSpanFinisher = noOpSpanFinisherType{}

// Helper functions for common attributes

func StringAttr(key, value string) Attribute {
	return Attribute{Key: key, Value: value}
}

func IntAttr(key string, value int) Attribute {
	return Attribute{Key: key, Value: value}
}

func Int64Attr(key string, value int64) Attribute {
	return Attribute{Key: key, Value: value}
}

func Float64Attr(key string, value float64) Attribute {
	return Attribute{Key: key, Value: value}
}

func BoolAttr(key string, value bool) Attribute {
	return Attribute{Key: key, Value: value}
}
