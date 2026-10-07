### Changed

- Files that belong to no chat are removed after a week, instead of
  piling up forever. That covers files an automation run, the API, or an
  MCP client made, and uploads never sent with a message. Files in a chat
  stay with the chat. The API shows when such a file goes as `expires_at`.
