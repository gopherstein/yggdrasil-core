### Added

- Report this, beside errors in chat, on a page that couldn't load or ran
  into a problem, in automation runs, and when a model won't start or
  install (#588). It shows everything the report sends: what went wrong
  and where, the Toskar version, the kind of computer, and for an Admin
  Toskar's last warnings and errors. You can add what you were doing.
  Email addresses, keys, home folder names, the computer's name, and
  internet addresses are taken out first. Send by email opens your mail
  app addressed to info@yeix.io, and Copy is there if email isn't set up.
- `GET /api/v1/diagnostics/report` (contract 1.32) gives that report,
  scrubbed. Members may call it; only an Admin's report has the log.
