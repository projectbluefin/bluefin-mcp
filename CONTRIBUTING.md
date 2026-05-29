# Contributing to bluefin-mcp

## This is an experiment

bluefin-mcp is an early-stage experiment exploring whether a semantic MCP
server can give AI agents meaningful context about a Bluefin system. Design
choices are provisional. Approach contributions with curiosity rather than
certainty — we are learning what works.

## Design principles

### Read-only by default

The server is read-only and local-only. The only write operation permitted
is `store_unit_docs`, which writes to the user's local machine. No other
writes are allowed without explicit human approval.

### No automated reporting

Under no circumstances should any tool send data, telemetry, bug reports,
or automated issues to any external service. The human decides what to do
with information the tool surfaces.

### Server boundary

Before designing any new tool, check what
[linux-mcp-server](https://github.com/rhel-lightspeed/linux-mcp-server)
already covers:

| Concern | Covered by |
|---|---|
| CPU, memory, disk, full hardware inventory | linux-mcp-server |
| journalctl, service status, process list, network | linux-mcp-server |
| Bluefin semantics, variant state, ujust, compatibility layer | **bluefin-mcp** |

Do not duplicate linux-mcp-server functionality.

### sysfs-first for hardware reads

Prefer `/sys/class/dmi/id/` over `dmidecode`. sysfs is world-readable;
dmidecode requires root. This server runs unprivileged.

## Development workflow

1. Fork the repo and clone locally.
2. Create a branch: `git checkout -b feat/your-feature`.
3. Write tests alongside your changes.
4. Run `just build` and `go test ./...` before pushing.
5. Open a PR against `main`.

## Code style

- Follow standard Go conventions (`go fmt`, `go vet`).
- Keep tools focused: one MCP tool = one clear responsibility.
- Error messages should help an AI agent (or human) diagnose the problem
  without reading source code.
