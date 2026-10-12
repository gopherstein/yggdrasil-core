package artifacts

import (
	"context"
	"testing"
)

// How a file was made is kept with it and comes back when it's read or
// listed (#553); a file without any has none.
func TestDetailsAreKept(t *testing.T) {
	s, _ := newStore(t)
	ctx := context.Background()
	made, err := s.Save(ctx, Input{ConversationID: "c1", Name: "dog.png", Producer: ProducerAssistant, Data: []byte("\x89PNG"),
		Details: map[string]any{"model": "Z-Image-Turbo", "seed": int64(42), "prompt": "a dog"}})
	if err != nil {
		t.Fatal(err)
	}
	plain, err := s.Save(ctx, Input{ConversationID: "c1", Name: "notes.md", Producer: ProducerUser, Data: []byte("hi")})
	if err != nil {
		t.Fatal(err)
	}
	got, _, err := s.Read(ctx, made.ID)
	if err != nil || got.Details["model"] != "Z-Image-Turbo" || got.Details["seed"] != float64(42) || got.Details["prompt"] != "a dog" {
		t.Fatalf("read %+v %v", got.Details, err)
	}
	list, err := s.List(ctx, "c1")
	if err != nil || len(list) != 2 {
		t.Fatalf("list %v %v", list, err)
	}
	for _, a := range list {
		if a.ID == plain.ID && a.Details != nil {
			t.Fatalf("a plain file has details: %v", a.Details)
		}
		if a.ID == made.ID && a.Details["model"] != "Z-Image-Turbo" {
			t.Fatalf("listed without details: %v", a.Details)
		}
	}
}
