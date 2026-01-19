// Package monitoring provides distributed tracing for the RSS feed backend
package monitoring

import (
	"context"
	"fmt"
	"log"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
)

// InitTracing initializes OpenTelemetry tracing with console exporter for simplicity
func InitTracing(serviceName string) (*sdktrace.TracerProvider, error) {
	// Create a simple tracer provider with console exporter
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(serviceName),
		)),
	)

	// Register tracer provider
	otel.SetTracerProvider(tp)

	return tp, nil
}

// ShutdownTracing shuts down the tracer provider
func ShutdownTracing(tp *sdktrace.TracerProvider) {
	if err := tp.Shutdown(context.Background()); err != nil {
		log.Printf("Error shutting down tracer provider: %v", err)
	}
}

// CreateSpan creates a new span with the given name
func CreateSpan(ctx context.Context, name string) (context.Context, trace.Span) {
	tracer := otel.Tracer("rss-feed-backend")
	return tracer.Start(ctx, name)
}

// SetSpanAttributes sets attributes on the given span
func SetSpanAttributes(span trace.Span, attributes map[string]interface{}) {
	var attrs []attribute.KeyValue
	for k, v := range attributes {
		attrs = append(attrs, attribute.String(k, fmt.Sprintf("%v", v)))
	}
	span.SetAttributes(attrs...)
}

// SetSpanError sets error information on the given span
func SetSpanError(span trace.Span, err error) {
	span.SetAttributes(attribute.String("error", err.Error()))
	span.RecordError(err)
}

// AddSpanEvent adds an event to the given span
func AddSpanEvent(span trace.Span, eventName string, attributes map[string]interface{}) {
	var attrs []attribute.KeyValue
	for k, v := range attributes {
		attrs = append(attrs, attribute.String(k, fmt.Sprintf("%v", v)))
	}
	span.AddEvent(eventName, trace.WithAttributes(attrs...))
}
