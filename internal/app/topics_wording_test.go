package app

import (
	"strings"
	"testing"

	"github.com/yeixio/toskar-core/pkg/contracts"
)

// The check before an answer reads what a message is about, not how it
// asks: "Search the web:" in front of a tire question wrongly read as off
// topic on qwen2.5-7b in the quality run.
func TestSubjectOf(t *testing.T) {
	for in, want := range map[string]string{
		"Search the web: are all-season tires OK in snow?":  "are all-season tires OK in snow?",
		"Please search online for winter tire prices":       "winter tire prices",
		"can you look up the best tire pressure for winter": "the best tire pressure for winter",
		"google: alignment cost":                            "alignment cost",
		"What tire pressure should I run in winter?":        "What tire pressure should I run in winter?",
		"Searching for a good shop":                         "Searching for a good shop",
		"search the web":                                    "search the web",
	} {
		if got := subjectOf(in); got != want {
			t.Errorf("subjectOf(%q) = %q, want %q", in, got, want)
		}
	}
}

// A check's refusal sentence is used only in the reply's language: the
// quality run's model wrote one in Spanish for an English question.
func TestSameLanguage(t *testing.T) {
	spanish := "¿Puedo ayudar con algo relacionado con neumáticos, alineaciones o horarios de la tienda?"
	if sameLanguage(spanish, "en") {
		t.Error("a Spanish sentence counted as English")
	}
	if !sameLanguage(spanish, "es-419") || !sameLanguage("I can help with tires, wheels, and bookings at the shop.", "en") {
		t.Error("a sentence in the reply's language was refused")
	}
	if !sameLanguage("OK!", "de") {
		t.Error("a sentence too short to tell was refused")
	}
}

// The check leans toward answering when unsure, since every answer is
// checked again after it's written.
func TestTopicCheckLeansOnTopic(t *testing.T) {
	s := topicCheckSystem(&contracts.TopicPolicy{StaysOn: "Tires"}, "each message a person sends")
	if !strings.Contains(s, "When unsure between on_topic and off_topic, choose on_topic") || !strings.Contains(s, "caring for") {
		t.Errorf("system = %s", s)
	}
}
