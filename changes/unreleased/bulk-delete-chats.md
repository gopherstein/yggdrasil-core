### Added

- Apps can delete several chats in one request, `POST /api/v1/conversations/delete`,
  with their messages and files. Each person, paired device, and portal guest
  can delete only their own chats. The web app and the phone app will use it
  to select and delete chats together.

### Security

- Deleting a chat removes its files only when the chat is yours. Before, a
  request to delete someone else's chat failed but still removed that chat's
  files.
