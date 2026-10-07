package pluginapi

import (
	"encoding/json"
	"testing"
)

func TestChatMessageJSON(t *testing.T) {
	plain, _ := json.Marshal(ChatMessage{Role: "user", Content: "hi"})
	if string(plain) != `{"role":"user","content":"hi"}` {
		t.Fatalf("plain = %s", plain)
	}

	// A message with a picture is sent as OpenAI content parts (#191).
	withImage, _ := json.Marshal(ChatMessage{Role: "user", Content: "What is this?", Images: []string{"data:image/png;base64,AAAA"}})
	want := `{"role":"user","content":[{"type":"text","text":"What is this?"},{"type":"image_url","image_url":{"url":"data:image/png;base64,AAAA"}}]}`
	if string(withImage) != want {
		t.Fatalf("with image = %s", withImage)
	}

	// Both forms read back, so a paired computer or an API caller can send
	// either.
	var m ChatMessage
	if err := json.Unmarshal(withImage, &m); err != nil || m.Content != "What is this?" || len(m.Images) != 1 || m.Images[0] != "data:image/png;base64,AAAA" {
		t.Fatalf("parts = %+v %v", m, err)
	}
	if err := json.Unmarshal(plain, &m); err != nil || m.Content != "hi" || m.Images != nil {
		t.Fatalf("string = %+v %v", m, err)
	}
	if err := json.Unmarshal([]byte(`{"role":"user","content":[{"type":"text","text":"a"},{"type":"text","text":"b"}]}`), &m); err != nil || m.Content != "a\nb" {
		t.Fatalf("texts = %+v %v", m, err)
	}
	if err := json.Unmarshal([]byte(`{"role":"user","content":42}`), &m); err == nil {
		t.Fatal("a number was read as content")
	}
}
