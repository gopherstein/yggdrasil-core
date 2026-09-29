# Architecture

Yggdrasil Core is one daemon process, `yggdrasil-daemon`, plus the web UI it serves. Desktop and mobile applications are separate clients. They are not built from this repository.

```text
client (web UI, curl, another program, desktop, mobile)
        |
        |  HTTP  /api/v1 and /v1     default 127.0.0.1:7331
        v
  yggdrasil-daemon
        |
        +-- profiles, tasks, tools
        +-- model catalog and downloads
        +-- Norn placement
        |
        +-- runtime adapters
        |     +-- llamacpp (local llama-server)
        |     +-- external-openai (configured remote server)
        |
        +-- Bifrost  :7332
              discovery, pairing, paired-node calls
```

Local state is a SQLite database, model files, runtime binaries, logs, and a secrets directory under the OS data path. See [Privacy](privacy.md).

## Names used in the code

Some Norse names appear in comments and logs. Others are not packages in this repository. The table uses the names as they exist today.

| Name | Role people expect | In this repository | Status |
| --- | --- | --- | --- |
| Yggdrasil Core | Daemon, API, local web UI | `cmd/daemon`, `internal/app`, `internal/api`, `web/` | Implemented |
| Bifrost | Discovery, pairing, node protocol | `internal/discovery`, `internal/nodes`, `internal/auth` | Implemented |
| Norn | Scheduling and workload placement | `internal/scheduler` | Implemented |
| Heimdall | Health and diagnostics | `internal/events` calls its bus a Heimdall event stream. Diagnostics and model health live in `internal/diagnostics` and `internal/models/health`. | Partial |
| Huginn | Agent execution | No package uses this name. Chat and tasks run through the `simple` and `team` orchestrators. | Planned as a named subsystem. Orchestrators are implemented. |
| Muninn | Persistent memory and context | No package uses this name. Conversations, messages, tasks, and settings are rows in SQLite. | Planned as a named subsystem. Conversation storage is implemented. |
| Mimir | Knowledge and retrieval | No retrieval or document-index code. | Planned |
| Gungnir | Tool and task execution | No package uses this name. Tools are implemented in `internal/tools` (internet, filesystem, terminal, git). Tasks are implemented in `internal/tasks`. | Planned as a named subsystem. Tools and tasks are implemented. |

## Request path

1. A client calls `/api/v1/chat`, `/api/v1/tasks`, or `/v1/chat/completions`.
2. The app resolves a profile. Built-in profiles include General Assistant (`simple`), Programming (`team`), and Research (`simple`).
3. The task manager asks Norn where a role should run. Norn scores paired nodes that are online and have the model.
4. The chosen machine starts the model on a runtime adapter. The default local runtime is llama.cpp.
5. Tokens stream back as events. The web UI subscribes to `GET /api/v1/events`.

The OpenAI handler sends the last user message into that same chat path. It does not forward the rest of the message array. See [API](api.md).

## Bifrost

Bifrost is the internal HTTP server on port 7332.

- mDNS service type `_localai._tcp` on domain `local.`
- optional static peers (`YGGDRASIL_STATIC_PEERS` or `config.json`) when mDNS is not available, including Docker
- pairing offer and approval before a peer is trusted
- certificate-backed bearer tokens on protected routes such as remote chat and model control

Discovery is on by default. When it is on and the internal bind address is still loopback, startup rebinds Bifrost to `0.0.0.0` so peers on the LAN can connect. Pairing routes on that port are reachable without a token until a peer is trusted. Protected routes reject unsigned calls.

## Norn

Norn is a deterministic placer. Given the nodes the daemon knows about, which models they have, and a role, it picks a node and records a `scheduler.placement` event. It does not split one model across computers. Cross-machine work that exists today is placement of whole roles, as in the Team pipeline. User schedules are a separate daemon loop in `internal/automations`. Norn places the model for a due automation the same way it places a chat role.

## Orchestrators

| Id | Behavior | Status |
| --- | --- | --- |
| `simple` | One model, optional tool loop | Implemented |
| `team` | Coordinator, then worker, then reviewer | Implemented |

Team can place those roles on different paired computers. A manual script for that is [two-machine-team-demo.md](two-machine-team-demo.md).

## Heimdall, as far as it exists

The event bus publishes structured events for tasks, models, tools, nodes, placement, and chat tokens. `GET /api/v1/events` is a server-sent stream of that bus. `GET /api/v1/diagnostics` builds a zip that omits secrets. A model health monitor can stop a model that stops responding. There is no separate telemetry pipeline.

## Runtimes

Runtime adapters implement `pkg/pluginapi.Runtime`: detect, install, start, stop, and health. The process that actually generates tokens is outside the daemon (`llama-server`, or an HTTP server you already run). See [runtimes.md](runtimes.md).

## What is not in this process

- Yggdrasil Desktop and Yggdrasil Mobile
- retrieval-augmented generation
- training
- splitting a single model across machines
