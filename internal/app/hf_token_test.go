package app

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/yeixio/toskar-core/pkg/contracts"
)

// The Hugging Face token is set from Settings, never shown, refused when it
// isn't one, and forgotten when cleared; the picture setup reads it (#551).
func TestHuggingFaceToken(t *testing.T) {
	a, _, _ := pictureApp(t, false)
	ctx := context.Background()
	err := a.applySettingsPatch(ctx, map[string]any{"huggingface_token": "not a token"})
	if code, _ := contracts.ErrorCode(err); code != "HUGGINGFACE_TOKEN_INVALID" {
		t.Fatalf("a bad token: %v", err)
	}
	if err := a.applySettingsPatch(ctx, map[string]any{"huggingface_token": "  hf_abcdefghijklmnopqrstuvwxyz  "}); err != nil {
		t.Fatal(err)
	}
	view, err := a.settingsView(ctx)
	if err != nil || !view.HuggingFaceTokenSet {
		t.Fatalf("view %+v %v", view.HuggingFaceTokenSet, err)
	}
	if a.Images.Token() != "hf_abcdefghijklmnopqrstuvwxyz" || !a.Images.Status().TokenSet {
		t.Fatal("the picture setup doesn't see the token")
	}
	if raw, _ := json.Marshal(view); strings.Contains(string(raw), "hf_abc") {
		t.Fatal("the settings show the token")
	}
	if err := a.applySettingsPatch(ctx, map[string]any{"huggingface_token": ""}); err != nil {
		t.Fatal(err)
	}
	if view, _ := a.settingsView(ctx); view.HuggingFaceTokenSet || a.huggingFaceToken() != "" {
		t.Fatal("a cleared token is still set")
	}
}
