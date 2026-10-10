package app

import (
	"context"

	"github.com/yeixio/toskar-core/internal/auth"
	"github.com/yeixio/toskar-core/pkg/contracts"
)

// deleteConversation deletes one of the person's chats. The chat goes
// first, so its files go only when it was theirs: files are kept by chat,
// not by person.
func (a *App) deleteConversation(ctx context.Context, id string) error {
	if err := a.Conversations.Delete(ctx, id); err != nil {
		return err
	}
	if a.Artifacts != nil {
		return a.Artifacts.DeleteConversation(ctx, id)
	}
	return nil
}

// deleteConversations deletes the person's chats among ids at once (#452):
// the chats and their messages in one transaction, then each one's files.
// Another person's chat, or one that doesn't exist, is skipped as
// not_found, as a single delete would fail.
func (a *App) deleteConversations(ctx context.Context, ids []string) (contracts.ConversationsDeleted, error) {
	deleted, err := a.Conversations.DeleteMany(ctx, ids)
	if err != nil {
		return contracts.ConversationsDeleted{}, err
	}
	out := contracts.ConversationsDeleted{Deleted: deleted, Skipped: []contracts.ConversationSkipped{}}
	gone := map[string]bool{}
	for _, id := range deleted {
		gone[id] = true
		if a.Artifacts != nil {
			if err := a.Artifacts.DeleteConversation(ctx, id); err != nil {
				// The chat is gone; a file left behind is logged, not undone.
				a.Logger.Warn("chat files not removed", "conversation_id", id, "err", err)
			}
		}
	}
	for _, id := range ids {
		if !gone[id] {
			out.Skipped = append(out.Skipped, contracts.ConversationSkipped{ID: id, Reason: "not_found"})
		}
	}
	a.Logger.Info("chats deleted", "person_id", auth.PersonID(ctx), "deleted", len(deleted), "skipped", len(out.Skipped))
	return out, nil
}
