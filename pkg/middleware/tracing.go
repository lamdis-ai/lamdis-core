// pkg/middleware/tracing.go
package middleware

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"lamdis/pkg/config"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

var (
	inited       bool
	instrumented bool
)

func Tracing(cfg config.Config) func(http.Handler) http.Handler {
	if !inited {
		// Only initialize OTLP exporter if explicitly configured via env.
		endpoint := os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT")
		if endpoint == "" {
			endpoint = os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
		}
		if endpoint != "" {
			opts := []otlptracehttp.Option{}
			if strings.HasPrefix(strings.ToLower(endpoint), "http://") {
				opts = append(opts, otlptracehttp.WithInsecure())
			}
			if exp, err := otlptracehttp.New(context.Background(), opts...); err == nil {
				if res, err := resource.New(context.Background(), resource.WithAttributes(semconv.ServiceName("lamdis-gateway"))); err == nil {
					tp := trace.NewTracerProvider(trace.WithBatcher(exp), trace.WithResource(res))
					otel.SetTracerProvider(tp)
					instrumented = true
				} else {
					fmt.Printf("tracing: resource init failed: %v\n", err)
				}
			} else {
				fmt.Printf("tracing: exporter init failed (will disable instrumentation): %v\n", err)
			}
		}
		inited = true
	}
	// If not instrumenting, return pass-through middleware to avoid otelhttp wrapper
	if !instrumented {
		return func(next http.Handler) http.Handler { return next }
	}
	return func(next http.Handler) http.Handler { return otelhttp.NewHandler(next, "http") }
}
