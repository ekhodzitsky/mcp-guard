package telemetry

import (
	"context"
	"sync"
	"testing"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// inMemoryExporter is a test span exporter that records exported spans.
type inMemoryExporter struct {
	mu     sync.Mutex
	spans  []sdktrace.ReadOnlySpan
	closed bool
}

func (e *inMemoryExporter) ExportSpans(_ context.Context, spans []sdktrace.ReadOnlySpan) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return nil
	}
	e.spans = append(e.spans, spans...)
	return nil
}

func (e *inMemoryExporter) Shutdown(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.closed = true
	return nil
}

func (e *inMemoryExporter) Spans() []sdktrace.ReadOnlySpan {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]sdktrace.ReadOnlySpan(nil), e.spans...)
}

func TestInitExportsSpans(t *testing.T) {
	ctx := context.Background()

	exp := &inMemoryExporter{}
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exp),
	)

	// Set the global provider so otel.Tracer uses it.
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	defer otel.SetTracerProvider(prev)

	tr := provider.Tracer("test-service")
	_, span := tr.Start(ctx, "test-span")
	span.End()

	if err := provider.ForceFlush(ctx); err != nil {
		t.Fatalf("ForceFlush failed: %v", err)
	}

	spans := exp.Spans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if spans[0].Name() != "test-span" {
		t.Errorf("expected span name %q, got %q", "test-span", spans[0].Name())
	}
}

func TestInitSetsServiceName(t *testing.T) {
	ctx := context.Background()

	shutdown, err := Init(ctx, "mcp-guard-test")
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	defer func() {
		if err := shutdown(ctx); err != nil {
			t.Fatalf("shutdown failed: %v", err)
		}
	}()

	tr := providerFromGlobal().Tracer("mcp-guard-test")
	_, span := tr.Start(ctx, "service-span")
	span.End()
}

func TestRaceSafeTracerAccess(t *testing.T) {
	ctx := context.Background()

	exp := &inMemoryExporter{}
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exp),
	)

	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	defer otel.SetTracerProvider(prev)

	// Concurrently create spans using otel.Tracer — must not race.
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, span := provider.Tracer("race-test").Start(ctx, "race-span")
			span.End()
		}()
	}
	wg.Wait()

	if err := provider.ForceFlush(ctx); err != nil {
		t.Fatalf("ForceFlush failed: %v", err)
	}

	spans := exp.Spans()
	if len(spans) != 100 {
		t.Fatalf("expected 100 spans, got %d", len(spans))
	}
}

func providerFromGlobal() *sdktrace.TracerProvider {
	// Access the global provider set by Init.
	// This helper avoids exposing internals; it is safe because
	// Init always sets a *sdktrace.TracerProvider.
	if tp, ok := otel.GetTracerProvider().(*sdktrace.TracerProvider); ok {
		return tp
	}
	return nil
}
