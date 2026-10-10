### Added

- Add a model from a GGUF file through the API, `POST /api/v1/models/import`:
  a file on this computer, copied into Toskar's models folder or used where
  it is, or the file itself sent from another device. The file's header is
  checked first, so an incomplete copy or a file that isn't a model is
  refused, and the model's name, size, quantization, and context length come
  from the file. The web app and the phone app will use it next.

### Fixed

- Deleting a model only deletes files in Toskar's models folder.
