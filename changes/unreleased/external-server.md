### Added

- Connect an OpenAI-compatible server, such as OpenAI, or vLLM or Ollama on another machine, in Settings → External server (advanced mode). Its models appear for a chat to choose, marked external, and chats with them are recorded in What left this computer. Auto never picks them, and offline profiles and chats using memories or knowledge marked This computer only refuse them.

### Fixed

- The `external-openai` runtime could not be configured, so it always reported "not configured" though the documentation described it as working.
