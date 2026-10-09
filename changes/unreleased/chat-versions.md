### Added

- Chats keep versions: retrying an answer or editing a sent message adds a
  version of that point instead of replacing it, with the conversation
  that followed each version kept. The API answers from any point
  (`retry_of`, `edit_of`, `parent_id` on `POST /api/v1/chat`), lists each
  message's `versions`, and switches the one shown; the model is sent only
  the chat up to that point. Existing chats read as one version at every
  point.
