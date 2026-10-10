package guide

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestAboutYggdrasil(t *testing.T) {
	cases := map[string]string{
		"How do I schedule an automation?":        "Schedule an automation",
		"Can Yggdrasil train its own AI?":         "Train your own AI",
		"what features do you have?":              "What Toskar does",
		"How do I connect my other computer?":     "Connect your computers",
		"where is what left this computer?":       "Privacy, encryption, and what left this computer",
		"is my data encrypted?":                   "Privacy, encryption, and what left this computer",
		"are my chats private?":                   "Privacy, encryption, and what left this computer",
		"Do I need encryption?":                   "Privacy, encryption, and what left this computer",
		"who can see my chats?":                   "Privacy, encryption, and what left this computer",
		"How do I add knowledge from a folder?":   "Connect knowledge",
		"How do I use an API key with Yggdrasil?": "Connect another app through the API",
		"how to turn on notifications by email?":  "Notifications",
		"what does the Team profile do?":          "Run a Team profile",
		"How do I reach Toskar away from home?":   "Reach Toskar away from home",
		"How do I compare independent drafts?":    "Compare independent drafts",
		"what does the Judge role do?":            "Compare independent drafts",
		// The product's name, and the one from before the rename (#237).
		"Can Toskar train its own AI?":         "Train your own AI",
		"How do I use an API key with Toskar?": "Connect another app through the API",
	}
	for q, want := range cases {
		ps := About(q)
		if len(ps) == 0 || ps[0].Section != want {
			t.Errorf("About(%q) = %v, want %s first", q, ps, want)
		}
	}
	// Asking what it can do counts as naming it; a question that merely
	// contains "you have" ("until you have taken them all") doesn't.
	if ps := About("which runtimes do you support?"); len(ps) == 0 {
		t.Error("About(which runtimes do you support?) found nothing")
	}
	for _, q := range []string{
		"How do I make pasta carbonara?", "Can you write a poem about the sea?", "What is the capital of France?",
		"How do I fix a flat bike tire?", "Explain quantum computing", "How do I improve my resume?",
		"what is memory in a computer?", "Summarize this: the meeting moved to Tuesday.",
		"How does AES encryption work?", "Is end-to-end encryption safe?",
		// Long questions sharing a few common words with a passage.
		"A doctor gives you 3 pills and tells you to take one every half hour, starting now. How many minutes until you have taken them all?",
		"A farmer has 17 sheep. All but 9 run away. How many sheep are left?",
		"How many months of the year have 28 days?",
	} {
		if ps := About(q); len(ps) != 0 {
			t.Errorf("About(%q) = %v, want nothing", q, ps)
		}
	}
}

// TestAboutReasoning: none of the quality set's reasoning questions is about
// Toskar, asked plainly or with the set's "Final answer" request.
func TestAboutReasoning(t *testing.T) {
	raw, err := os.ReadFile("../../tests/quality/reasoning.json")
	if err != nil {
		t.Fatal(err)
	}
	var file struct {
		Cases []struct{ ID, Message string } `json:"cases"`
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatal(err)
	}
	for _, c := range file.Cases {
		for _, q := range []string{c.Message, c.Message + "\n\nGive your final answer on the last line, as: Final answer: …"} {
			if ps := About(q); len(ps) != 0 {
				t.Errorf("%s: About = %s, want nothing", c.ID, ps[0].Section)
			}
		}
	}
}

func TestBlock(t *testing.T) {
	ps := About("How do I schedule an automation?")
	b := Block(ps)
	if !strings.Contains(b, "[Schedule an automation]") || !strings.Contains(b, "say so instead of guessing") {
		t.Fatalf("block = %q", b)
	}
	total := 0
	for _, p := range ps {
		total += len(p.Text)
	}
	if total > maxChars+2000 {
		t.Fatalf("too much guide: %d characters", total)
	}
	if Block(nil) != "" {
		t.Fatal("empty block")
	}
}
