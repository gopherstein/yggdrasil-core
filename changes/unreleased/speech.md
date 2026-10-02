### Added

- Speech on this computer: `speech.transcribe` writes down what an attached audio file says (Whisper, fast or accurate), and `speech.synthesize` reads text aloud as an audio file (Piper). Answers have a Read aloud button (`POST /api/v1/speech`). Chats accept audio files (`.wav`, `.mp3`, `.m4a`, `.aac`, `.ogg`, `.flac`, `.webm`), which play in the chat. The first use installs speech from PyPI and downloads the model or voice from Hugging Face; audio and text are not sent anywhere.
- A Speech capability in profiles turns transcription and reading aloud on or off. Offline profiles keep it.
