### Added

- Email and calendar with your own accounts and an app password (no Google or Microsoft sign-in app). Email (IMAP and SMTP: Fastmail, iCloud, Proton Mail Bridge, Nextcloud, your own server, and Gmail or Outlook where app passwords are allowed) adds `email.search`, `email.read` (without marking messages read), and, asking first, `email.draft`, `email.send` (threaded replies, a copy in Sent), and `email.archive`. Nothing is deleted. Calendar (CalDAV) adds `calendar.search` and `calendar.availability`, and, asking first, `calendar.create`, `calendar.update`, and `calendar.cancel`, which keep attendees and alarms and never overwrite an event changed elsewhere. Connect them in Settings → Connected services.

### Fixed

- Connected services scrub only secret values, such as a token, from results. Every stored value of six or more characters was scrubbed, so a result mentioning the service's own address, such as a Home Assistant URL, read "[redacted]".
