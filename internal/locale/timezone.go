package locale

import (
	"context"
	"time"
)

type timeZoneKey struct{}

// WithTimeZone carries the person's time zone for a request, as an IANA
// name such as America/Juneau from their browser. An empty or unknown name
// leaves this computer's own.
func WithTimeZone(ctx context.Context, name string) context.Context {
	if name == "" {
		return ctx
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return ctx
	}
	return context.WithValue(ctx, timeZoneKey{}, loc)
}

// TimeZone is the person's time zone for a request, or this computer's.
func TimeZone(ctx context.Context) *time.Location {
	if ctx != nil {
		if loc, ok := ctx.Value(timeZoneKey{}).(*time.Location); ok {
			return loc
		}
	}
	return time.Local
}
