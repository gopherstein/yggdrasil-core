package artifacts

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A file that belongs to no chat expires after a week; one attached to a
// chat, or saved in one, stays (#191).
func TestUnfiledFilesExpire(t *testing.T) {
	s, dir := newStore(t)
	ctx := context.Background()
	start := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return start }

	inChat, _ := s.Save(ctx, Input{ConversationID: "c1", Name: "kept.md", Data: []byte("a")})
	made, _ := s.Save(ctx, Input{Name: "chart.png", Producer: ProducerAssistant, Data: []byte("b")})
	sent, _ := s.Save(ctx, Input{Name: "upload.md", Data: []byte("c")})
	if inChat.ExpiresAt != nil {
		t.Fatalf("a file in a chat expires: %v", inChat.ExpiresAt)
	}
	if made.ExpiresAt == nil || !made.ExpiresAt.Equal(start.Add(UnfiledTTL)) {
		t.Fatalf("expires = %v", made.ExpiresAt)
	}
	if got, _ := s.Get(ctx, made.ID); got.ExpiresAt == nil || !got.ExpiresAt.Equal(start.Add(UnfiledTTL)) {
		t.Fatalf("stored expiry = %v", got.ExpiresAt)
	}

	// Sent with a message: it is the chat's now.
	if err := s.Attach(ctx, sent.ID, "c1"); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Get(ctx, sent.ID); got.ExpiresAt != nil {
		t.Fatalf("an attached file still expires: %v", got.ExpiresAt)
	}

	if n, err := s.RemoveExpired(ctx); err != nil || n != 0 {
		t.Fatalf("removed %d early, %v", n, err)
	}
	s.now = func() time.Time { return start.Add(UnfiledTTL + time.Minute) }
	if n, err := s.RemoveExpired(ctx); err != nil || n != 1 {
		t.Fatalf("removed %d, %v", n, err)
	}
	if _, err := s.Get(ctx, made.ID); err != ErrNotFound {
		t.Fatalf("expired file still listed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "unfiled", made.ID+".png")); !os.IsNotExist(err) {
		t.Fatal("expired file left on disk")
	}
	for _, id := range []string{inChat.ID, sent.ID} {
		if _, _, err := s.Read(ctx, id); err != nil {
			t.Fatalf("a chat's file was removed: %v", err)
		}
	}
}

// The migration's expiry for files already unfiled, written by SQLite,
// reads back as a time.
func TestSQLiteExpiryReads(t *testing.T) {
	s, _ := newStore(t)
	ctx := context.Background()
	a, _ := s.Save(ctx, Input{Name: "old.md", Data: []byte("x")})
	if _, err := s.db.ExecContext(ctx, `UPDATE artifacts SET expires_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now', '+7 days') WHERE id = ?`, a.ID); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(ctx, a.ID)
	if err != nil || got.ExpiresAt == nil || time.Until(*got.ExpiresAt) < 6*24*time.Hour {
		t.Fatalf("expires = %v, %v", got.ExpiresAt, err)
	}
}
