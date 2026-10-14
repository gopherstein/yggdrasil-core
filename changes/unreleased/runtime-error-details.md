### Changed

- When llama.cpp can't be installed, the error says which build it looked
  for, which releases it checked, and GitHub's HTTP status (with a hint
  when it's GitHub's rate limit), and where to put llama-server by hand
  (#64). docs/runtimes.md has the steps.
- Model health events carry the end of what the process wrote
  (`stderr_tail`) and the last health probe's error (`last_probe`), so a
  diagnostics subscriber can show why a model stopped (#9).
