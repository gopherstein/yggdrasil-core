# Roadmap

This list is directional. It is not a commitment and it has no dates. Shipped behavior is described in the [README](README.md) and [docs/architecture.md](docs/architecture.md).

## Now

- llama.cpp runtime installs and GGUF model lifecycle
- hardware inventory on macOS, Windows, and Linux
- Bifrost discovery, pairing, and paired-node calls
- Norn placement for Team roles
- loopback OpenAI-compatible chat and model list
- local web UI
- Linux packages, macOS headless archives, checksums, and a Windows amd64 archive in the release script
- tests, lint, `govulncheck`, and cross-compiled CI builds

## Next

- try the Windows amd64 archive on a clean Windows machine
- code signing for core release artifacts
- a `yggctl` that can show status, nodes, and models (today it prints version and paths)
- forward temperature, max token limits, and prior messages through `/v1/chat/completions`
- TLS or mTLS for remote API access. A bearer token on plain HTTP does not encrypt traffic.
- Windows llama.cpp install that can select a GPU build when one exists
- more hardware reports so [docs/compatibility.md](docs/compatibility.md) can move cells off Untested
- a short demo of startup, discovery, a model, an API call, and Norn placement

## Research

These are areas to explore. They are not scheduled.

- running one model across more than one machine
- distributed training
- more runtimes and hardware backends
- richer orchestration than the simple and Team pipelines
- named Huginn, Muninn, Mimir, and Gungnir subsystems (agent memory, retrieval, and a dedicated tool plane). Orchestrators, SQLite history, and tools exist today under other names. See [docs/architecture.md](docs/architecture.md).
