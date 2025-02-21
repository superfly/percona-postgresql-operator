package sentry

import (
	"context"
	"os"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"

	"github.com/percona/percona-postgresql-operator/internal/logging"
)

const (
	// DefaultTimeout is the default timeout for flushing events to Sentry
	DefaultTimeout = 2 * time.Second
)

// Initialize sets up the Sentry client with the provided DSN
func Initialize(dsn string) error {
	if dsn == "" {
		return nil // Sentry is disabled if no DSN is provided
	}

	err := sentry.Init(sentry.ClientOptions{
		Dsn:              dsn,
		EnableTracing:    true,
		TracesSampleRate: 1.0,
		Debug:            os.Getenv("SENTRY_DEBUG") == "true",
		Environment:      os.Getenv("SENTRY_ENVIRONMENT"),
	})

	if err != nil {
		return errors.Wrap(err, "failed to initialize Sentry")
	}

	return nil
}

// CaptureError reports an error to Sentry with additional context
func CaptureError(ctx context.Context, err error, tags map[string]string) {
	if err == nil {
		return
	}

	log := logging.FromContext(ctx)

	hub := sentry.GetHubFromContext(ctx)
	if hub == nil {
		hub = sentry.CurrentHub()
	}

	// Add any context from the logger
	if len(tags) > 0 {
		hub.ConfigureScope(func(scope *sentry.Scope) {
			for k, v := range tags {
				scope.SetTag(k, v)
			}
		})
	}

	hub.CaptureException(err)
	log.Error(err, "error captured by Sentry")
}

// RecoverPanic captures panics and reports them to Sentry
func RecoverPanic() {
	if err := recover(); err != nil {
		sentry.CurrentHub().Recover(err)
		sentry.Flush(DefaultTimeout)
		panic(err) // Re-panic after reporting
	}
}

// WithContext returns a new context with Sentry hub
func WithContext(ctx context.Context, namespacedName types.NamespacedName) context.Context {
	hub := sentry.CurrentHub().Clone()
	hub.ConfigureScope(func(scope *sentry.Scope) {
		scope.SetTag("namespace", namespacedName.Namespace)
		scope.SetTag("name", namespacedName.Name)
	})
	return sentry.SetHubOnContext(ctx, hub)
}

// Flush ensures all queued events are sent to Sentry
func Flush() {
	sentry.Flush(DefaultTimeout)
}