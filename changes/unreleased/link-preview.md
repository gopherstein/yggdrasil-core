### Changed

- **From a link** no longer starts a download right away (#611, reported
  by @innovagent). Checking the link first shows what it would download:
  the size, the space left, and that it's a direct download saved on this
  computer, which Toskar doesn't charge for. When LM Studio, Ollama,
  llama.cpp, or GPT4All already has the same file, it offers **Use it
  where it is**: no download and no extra space. It won't start a second
  copy that doesn't fit.
- **From another app** comes first in Add a model when another app has
  models Toskar could use.
