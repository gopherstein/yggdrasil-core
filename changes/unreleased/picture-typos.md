### Fixed

- A request for a picture with the word misspelled, such as "make a
  picutre of a dog", makes the picture. When a request is worded in a way
  Toskar doesn't recognize and the model says it can't make pictures, or
  points to DALL-E, Midjourney, or ASCII art, Toskar makes the picture
  anyway.
- "Make" in a message no longer offers the terminal unless it means the
  build tool ("make test", a Makefile), so "make a picture of a dog" can't
  turn into a command that draws ASCII art.
