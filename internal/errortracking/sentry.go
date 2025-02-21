package errortracking

import (
	"github.com/getsentry/sentry-go"
)

// CaptureError captures an error and sends it to Sentry with additional context
func CaptureError(err error, tags map[string]string) {
	if err == nil {
		return
	}

	event := sentry.NewEvent()
	event.Message = err.Error()
	event.Level = sentry.LevelError
	event.Tags = tags

	sentry.CaptureEvent(event)
}

// CaptureException is a simpler version that just captures the error
func CaptureException(err error) {
	if err == nil {
		return
	}
	sentry.CaptureException(err)
}