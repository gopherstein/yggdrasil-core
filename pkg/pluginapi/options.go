package pluginapi

import "context"

// GenerateOptions limit one model call: a sampling temperature and a cap on
// the reply's tokens (#459, docs/deliberate.md). Zero leaves the model's
// defaults. They ride on the call's context, so an orchestrator sets them
// for one Generate without changing ExecutionEnvironment, and the call
// carries them to whichever computer runs it.
type GenerateOptions struct {
	Temperature float64
	MaxTokens   int
}

type generateOptionsKey struct{}

// WithGenerateOptions sets the options for calls made with ctx.
func WithGenerateOptions(ctx context.Context, o GenerateOptions) context.Context {
	return context.WithValue(ctx, generateOptionsKey{}, o)
}

// GenerateOptionsFrom is the options set on ctx, or none.
func GenerateOptionsFrom(ctx context.Context) GenerateOptions {
	o, _ := ctx.Value(generateOptionsKey{}).(GenerateOptions)
	return o
}
