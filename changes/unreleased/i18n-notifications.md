### Added

- Notifications in the API carry `message`, their title and body as translation keys with the values they need, and webhooks also get `language`. The client contract is 1.4.

### Changed

- Notifications show in the App language: the bell writes each notice in the language you chose, and desktop notices, email, push, and webhooks are written in the App language when they are sent. Text a model or an automation wrote stays as it was written.
