### Fixed

- A recording attached to a message is transcribed before the answer is
  written, so "what does this say?" gets the words instead of an empty
  reply. A reply that's only a tool call that can't run is asked for again
  in plain text, never shown empty.
