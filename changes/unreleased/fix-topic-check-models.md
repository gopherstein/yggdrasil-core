### Fixed

- Gemma 2 9B, Qwen 2.5 14B, and Qwen 2.5 32B now run a profile's topic
  checks when they're installed, as Gemma 3 4B does: they passed the topic
  quality set, and smaller models wrongly refused real questions.
- Toskar recognizes Spanish more reliably, so an answer or a topic refusal
  is in the person's language instead of being mistaken for Portuguese.
