### Changed

- Enforce's topic checks run on the smallest installed model verified to
  hold them (Gemma 3 4B so far) when it fits beside the answering model,
  instead of always on the answering model. Diagnostics and the Topics tab
  say when no verified model is installed.
