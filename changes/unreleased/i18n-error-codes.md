### Changed

- Errors from Yggdrasil show in the App language. The service sends each error with a stable code, such as `MODEL_NOT_INSTALLED` or `MEMORY_LOOKS_SECRET`, and the values its message needs, and the web UI shows the text for the code; the English text stays under Details for Diagnostics. Chat recognizes errors such as a model running out of memory or a computer going offline by their code instead of by their English wording. Setting an invalid value in Settings now says which setting and value, instead of "Could not update settings."
- The client contract is 1.3: a chat that fails while streaming sends `event: error_code` with the code, before `event: error` with the text, which older apps still read.
