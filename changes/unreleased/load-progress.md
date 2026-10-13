### Changed

- While a model loads for an answer, chat says which model, on which
  computer, and about how long it took last time ("Loading Qwen 2.5 7B on
  Office, about 20 s"), and no longer says "Loading" when the model is
  already running (#498). A load on a paired computer now counts as load
  time in run traces and in Diagnostics' first-answer times.
