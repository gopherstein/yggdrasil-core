### Added

- Apps can delete several chats in one request, `POST /api/v1/conversations/delete`,
  with their messages and files. Each person, paired device, and portal guest
  can delete only their own chats. The web app and the phone app will use it
  to select and delete chats together.
- Chat history has Select: tick several chats, or Shift-click a range, and
  delete them with one confirmation. Deleted chats, one or several, can be
  brought back with Undo for 10 seconds. Clearing history in Settings
  deletes in a few requests instead of one per chat.

### Security

- Deleting a chat removes its files only when the chat is yours. Before, a
  request to delete someone else's chat failed but still removed that chat's
  files.
