package app

import (
	"context"
	"strings"
	"testing"
)

// The answer's language (spec §11): a request wins, then the setting's
// mode decides, and the conversation fills in for short messages.
func TestReplyLanguageInTurnInstructions(t *testing.T) {
	t.Setenv("LANG", "")
	t.Setenv("LC_ALL", "")
	t.Setenv("LC_MESSAGES", "")
	a, conv := memoryApp(t)
	ctx := context.Background()
	env := &chatExecEnv{app: a, conversationID: conv}
	instructions := func(message string) string {
		env.turnPrompt = message
		return env.TurnInstructions(ctx, "a worker's prompt, written by the planner in English")
	}
	set := func(k, v string) {
		if err := a.Settings.Set(ctx, k, v); err != nil {
			t.Fatal(err)
		}
	}

	// Auto, the default: the language the person writes in.
	if got := instructions("Wie hoch ist die Zugspitze eigentlich genau?"); !strings.Contains(got, "Write your reply in German") {
		t.Fatalf("auto, German message:\n%s", got)
	}
	// A short message takes the conversation's language.
	if _, err := a.Conversations.AddMessage(ctx, conv, "user", "Quelle est la plus haute montagne de France ?"); err != nil {
		t.Fatal(err)
	}
	if got := instructions("ok, et après ?"); !strings.Contains(got, "in French") {
		t.Fatalf("auto, short message in a French conversation:\n%s", got)
	}
	// An explicit request wins in every mode.
	set("assistant_language_mode", "language")
	set("assistant_language", "es")
	if got := instructions("What is the tallest mountain in Europe? Answer in Japanese."); !strings.Contains(got, "in Japanese") {
		t.Fatalf("request:\n%s", got)
	}
	// A chosen language is used even when the person writes in another.
	if got := instructions("What is the tallest mountain in Europe?"); !strings.Contains(got, "in Spanish") {
		t.Fatalf("chosen language:\n%s", got)
	}
	// Same as app.
	set("assistant_language_mode", "app")
	set("ui_locale", "ko")
	if got := instructions("What is the tallest mountain in Europe?"); !strings.Contains(got, "in Korean") {
		t.Fatalf("same as app:\n%s", got)
	}
}

func TestAssistantLanguageSettings(t *testing.T) {
	t.Setenv("YGGDRASIL_DISCOVERY_ENABLED", "false")
	a, err := New(Options{DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Shutdown(context.Background()) })
	ctx := context.Background()
	view, err := a.settingsView(ctx)
	if err != nil || view.AssistantLanguageMode != "auto" || view.AssistantLanguage != "" {
		t.Fatalf("default view %q %q, %v", view.AssistantLanguageMode, view.AssistantLanguage, err)
	}
	if err := a.applySettingsPatch(ctx, map[string]any{"assistant_language_mode": "language", "assistant_language": "pt-BR"}); err != nil {
		t.Fatal(err)
	}
	mode, _ := a.Settings.GetString(ctx, "assistant_language_mode", "")
	lang, _ := a.Settings.GetString(ctx, "assistant_language", "")
	if mode != "language" || lang != "pt-BR" {
		t.Fatalf("saved %q %q", mode, lang)
	}
	if err := a.applySettingsPatch(ctx, map[string]any{"assistant_language_mode": "sometimes"}); err == nil {
		t.Fatal("an unknown mode was accepted")
	}
	if err := a.applySettingsPatch(ctx, map[string]any{"assistant_language": "Portuguese!"}); err == nil {
		t.Fatal("a language that isn't a tag was accepted")
	}
}

func TestSystemLanguage(t *testing.T) {
	t.Setenv("LC_ALL", "")
	t.Setenv("LC_MESSAGES", "")
	t.Setenv("LANG", "de_DE.UTF-8")
	if got := systemLanguage(); got != "de_DE" {
		t.Errorf("LANG=de_DE.UTF-8 → %q", got)
	}
	t.Setenv("LANG", "C.UTF-8")
	if got := systemLanguage(); got != "" {
		t.Errorf("LANG=C.UTF-8 → %q", got)
	}
}
