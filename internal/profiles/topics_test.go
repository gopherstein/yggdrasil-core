package profiles

import (
	"context"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/yeixio/toskar-core/internal/store"
	"github.com/yeixio/toskar-core/pkg/contracts"
)

func TestTopics(t *testing.T) {
	p := Normalize(Profile{Name: "Shop", OrchestratorID: "simple", Topics: &contracts.TopicPolicy{
		StaysOn: "  Tires  ", Examples: []string{" Winter tires? ", "", "  "}, NeverDiscuss: []string{"politics"},
	}})
	if p.Topics == nil || p.Topics.StaysOn != "Tires" || len(p.Topics.Examples) != 1 {
		t.Fatalf("normalized %+v", p.Topics)
	}
	if empty := Normalize(Profile{Topics: &contracts.TopicPolicy{StaysOn: "  "}}); empty.Topics != nil {
		t.Fatal("topics with nothing to stay on")
	}
	for _, bad := range []contracts.TopicPolicy{
		{StaysOn: "Tires", Strictness: "maximum"},
		{StaysOn: strings.Repeat("x", 1001)},
		{StaysOn: "Tires", OffTopicReply: strings.Repeat("x", 501)},
		{StaysOn: "Tires", Examples: []string{strings.Repeat("x", 201)}},
	} {
		if err := ValidateTopics(&bad); err == nil {
			t.Errorf("accepted %+v", bad.Strictness)
		}
	}
	if err := ValidateTopics(&contracts.TopicPolicy{StaysOn: "Tires", Strictness: contracts.TopicsEnforce}); err != nil {
		t.Fatal(err)
	}
}

// A profile keeps its topic controls (#345).
func TestTopicsAreStored(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	m := NewManager(db.SQL)
	ctx := context.Background()
	topics := &contracts.TopicPolicy{StaysOn: "Tires and bookings", Examples: []string{"Winter tires?"}, NeverDiscuss: []string{"politics"},
		OffTopicReply: "Only tires here!", Strictness: contracts.TopicsEnforce}
	p, err := m.Create(ctx, Profile{Name: "Shop", OrchestratorID: "simple", Topics: topics})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p.Topics, topics) {
		t.Fatalf("stored %+v", p.Topics)
	}
	p.Topics = nil
	if err := m.Update(ctx, p); err != nil {
		t.Fatal(err)
	}
	if got, _ := m.Get(ctx, p.ID); got.Topics != nil {
		t.Fatalf("topics after removing them: %+v", got.Topics)
	}
}
