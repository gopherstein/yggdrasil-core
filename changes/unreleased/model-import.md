### Added

- Add a model from a GGUF file through the API, `POST /api/v1/models/import`:
  a file on this computer, copied into Toskar's models folder or used where
  it is, or the file itself sent from another device. The file's header is
  checked first, so an incomplete copy or a file that isn't a model is
  refused, and the model's name, size, quantization, and context length come
  from the file.
- Toskar finds models LM Studio, Ollama, llama.cpp, and GPT4All already
  downloaded on this computer (`GET /api/v1/models/import/found`), so they
  can be added in place without downloading them again.
- Models → Add a model: send a GGUF file from the browser, add one by its
  location on this computer (copied or used where it is), pick one another
  app already downloaded, or paste a link.

### Fixed

- Deleting a model only deletes files in Toskar's models folder.
