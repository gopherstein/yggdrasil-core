package locale

import "strings"

// Text is one piece of a message: a catalog key with the values it needs,
// such as {"key": "notifications:notices.modelReady"}, or literal text that
// is the same in every language, such as an automation's name or what a
// model wrote.
type Text struct {
	Key    string         `json:"key,omitempty"`
	Params map[string]any `json:"params,omitempty"`
	Text   string         `json:"text,omitempty"`
}

// Message is text core keeps to show later, in whatever language the
// reader uses (multilingual spec §22): a title, and a body of sentences.
type Message struct {
	Title Text   `json:"title"`
	Body  []Text `json:"body,omitempty"`
}

// Key is a Text for a catalog key.
func Key(key string, params map[string]any) Text { return Text{Key: key, Params: params} }

// Literal is a Text that is shown as it is.
func Literal(text string) Text { return Text{Text: text} }

// Render returns the text in lang.
func (t Text) Render(lang string) string {
	if t.Key == "" {
		return t.Text
	}
	return T(lang, t.Key, t.Params)
}

// Render returns the title and body in lang. Body sentences are joined with
// a space, or with nothing in Chinese and Japanese, which don't put spaces
// between sentences.
func (m Message) Render(lang string) (title, body string) {
	parts := make([]string, 0, len(m.Body))
	for _, t := range m.Body {
		if s := strings.TrimSpace(t.Render(lang)); s != "" {
			parts = append(parts, s)
		}
	}
	sep := " "
	if base, _, _ := strings.Cut(Resolve(lang), "-"); base == "zh" || base == "ja" {
		sep = ""
	}
	return strings.TrimSpace(m.Title.Render(lang)), strings.Join(parts, sep)
}
