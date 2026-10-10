package pluginapi

import (
	"context"
	"testing"
)

func TestGenerateOptions(t *testing.T) {
	if o := GenerateOptionsFrom(context.Background()); o != (GenerateOptions{}) {
		t.Fatalf("none set: %+v", o)
	}
	ctx := WithGenerateOptions(context.Background(), GenerateOptions{Temperature: 0.4, MaxTokens: 512})
	if o := GenerateOptionsFrom(ctx); o.Temperature != 0.4 || o.MaxTokens != 512 {
		t.Fatalf("set: %+v", o)
	}
}
