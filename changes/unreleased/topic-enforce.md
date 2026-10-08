### Added

- Enforce for topic controls: each message is checked before it's
  answered, and an off-topic one gets the set reply without running the
  full answer. Each answer is checked too, and one that went off topic
  anyway is replaced. The profile editor's Topics tab chooses Guide or
  Enforce, and the run trace records what the check found.
