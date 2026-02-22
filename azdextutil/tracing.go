package azdextutil

import (
	"context"
	"os"
)

type traceContextKey struct{}

// TraceContext holds W3C distributed trace context propagated from azd.
type TraceContext struct {
	TraceParent string
	TraceState  string
}

// SetupTracingFromEnv reads TRACEPARENT and TRACESTATE from the environment
// and stores them in the context. Extensions can retrieve these values
// to propagate trace context to downstream calls.
func SetupTracingFromEnv(ctx context.Context) context.Context {
	traceparent := os.Getenv("TRACEPARENT")
	if traceparent == "" {
		return ctx
	}

	tc := &TraceContext{
		TraceParent: traceparent,
		TraceState:  os.Getenv("TRACESTATE"),
	}

	return context.WithValue(ctx, traceContextKey{}, tc)
}

// GetTraceContext retrieves the TraceContext from the context, if present.
func GetTraceContext(ctx context.Context) *TraceContext {
	tc, _ := ctx.Value(traceContextKey{}).(*TraceContext)
	return tc
}
