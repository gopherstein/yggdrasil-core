### Added

- Tools on other computers: image generation and speech run on whichever paired computer can run them, preferring one whose GPU does the work, so a laptop without image generation can ask for an image and a paired workstation makes it. The asking computer keeps the approval, the audit, and the files; the other computer runs the work and keeps nothing. The chat profile's computer policy applies, an unreachable computer is skipped for the next, and Stop stops the work there. Each job sent is recorded in What left this computer.
- Diagnostics → Tools on each computer shows each computer's image and speech providers: ready, installing, failed, or not set up, and whether a GPU does the work (`GET /api/v1/tools/providers`).
