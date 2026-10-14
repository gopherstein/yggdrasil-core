### Fixed

- **Start** on the Models page no longer does nothing when llama.cpp isn't
  set up, such as after adding a model From a link without going through
  onboarding (#610, reported by @innovagent). Start now sets llama.cpp up
  first and shows "Starting…" while it works. If it can't, it says why
  under the model, and the first chat that needs a model does the same.
  Every start attempt and how it ended is now in the daemon log.
