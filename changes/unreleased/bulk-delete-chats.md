### Added

- Apps can delete several chats in one request, `POST /api/v1/conversations/delete`,
  with their messages and files. Each person, paired device, and portal guest
  can delete only their own chats. The web app and the phone app will use it
  to select and delete chats together.
- Chat history has Select: tick several chats, or Shift-click a range, and
  delete them with one confirmation. Deleted chats, one or several, can be
  brought back with Undo for 10 seconds. Clearing history in Settings
  deletes in a few requests instead of one per chat.
- Settings → History can delete chats last used more than 30, 90, or 365
  days ago, keeping pinned chats, and says how many before you confirm.
  Clearing all history is confirmed in the page, which also works in the
  desktop app.

### Security

- Deleting a chat removes its files only when the chat is yours. Before, a
  request to delete someone else's chat failed but still removed that chat's
  files.
