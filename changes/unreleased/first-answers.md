### Added

- Diagnostics shows how fast answers start: the wait for the first word
  over the last week, for each app, with the model already running and
  with one loading first (#498). Chat requests can name their app in
  `client`, and run traces keep it and whether the request came from
  outside the home network. `GET /api/v1/diagnostics/first-answers` has
  the numbers.
