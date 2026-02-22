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

// Deprecated: Use azdext.NewContext() from github.com/azure/azure-dev/cli/azd/pkg/azdext instead.
// azdext.NewContext() provides proper OpenTelemetry W3C trace context propagation.
//
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

// Deprecated: Use azdext.NewContext() from github.com/azure/azure-dev/cli/azd/pkg/azdext instead.
// azdext.NewContext() provides proper OpenTelemetry W3C trace context propagation.
//
// GetTraceContext retrieves the TraceContext from the context, if present.
func GetTraceContext(ctx context.Context) *TraceContext {
	tc, _ := ctx.Value(traceContextKey{}).(*TraceContext)
	return tc
}
