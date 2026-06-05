# Contributing to bluefin-mcp

## This is an experiment

We are figuring out whether the MCP-server-for-troubleshooting approach works. The design is intentionally opinionated and the surface area intentionally small. Contributions that expand the server's scope should be discussed before implementation. The bar for adding a new tool is high — each tool adds maintenance burden and surface area for bugs.

We bias toward documentation. If you can improve an existing unit doc, a scenario description, or a knowledge store entry, start there.

## Design rules

These are hard constraints. PRs that violate them will not be merged.

### 1. Read-only, local-only

The only write operation is `store_unit_docs`, which writes to the user's local machine. No other tool may write to disk, the network, or any external service.

### 2. No automated reporting

Under no circumstances should any tool send data, telemetry, bug reports, or automated issues to any external service. The human decides what to do with information the tool surfaces. No `gh issue create`, no HTTP POST to a bug tracker, no analytics endpoint.

### 3. Server boundary: bluefin-mcp vs linux-mcp-server

Before designing any new tool, check what [`linux-mcp-server`](https://github.com/redhat-et/linux-mcp-server) already covers:

| linux-mcp-server | bluefin-mcp |
|---|---|
| CPU, memory, disk, hardware inventory | Bluefin-specific semantics |
| journalctl, service status, process list | Variant state (DX, Nvidia, etc.) |
| Network state, DNS, connectivity | ujust recipe tooling |
| Package inventory (rpm) | Compatibility / migration layer |
| dmidecode, lspci, lsusb | Knowledge store: what Bluefin units *mean* |

If `linux-mcp-server` already provides the raw fact, do not duplicate it here. Focus on the layer above: what does this fact *mean* on a Bluefin system?

### 4. Sysfs-first for hardware reads

Prefer `/sys/class/dmi/id/` over `dmidecode`. sysfs is world-readable; dmidecode requires root. This MCP server runs unprivileged and should never need `sudo`.

### 5. No silent external dependencies

New tools must degrade gracefully when their dependency is absent. A missing `bootc` binary should produce an actionable error, not a crash.

## Development workflow

### Prerequisites

- Go 1.23+
- `just` (optional — for the `just build` recipe)
- `bootc` and `rpm-ostree` (optional — for live system testing)

### Build and test

```bash
just build          # or: go build -o bin/bluefin-mcp ./cmd/bluefin-mcp
go test ./...       # run all tests
go test -race ./... # run with race detector
```

### Project structure

```
cmd/bluefin-mcp/   — entry point
internal/
  cli/             — command execution abstraction
  seed/            — embedded knowledge store (systemd unit docs, scenarios)
  system/          — bootc, rpm-ostree status parsing
  tools/           — MCP tool handlers (registered in tools.go)
```

### Adding a new tool

1. Define the tool in `internal/tools/tools.go` using `Register()`
2. Add a handler function in the appropriate `internal/` package
3. Add tests covering the handler with a mock runner
4. If the tool reads system state, prefer sysfs over privileged commands
5. If the tool has an external dependency, handle its absence gracefully

## Agent contributions

AI agents are welcome to contribute. The same design rules apply. Before opening a PR:

- Read `AGENTS.md` for agent-specific instructions
- Follow the server boundary rule — do not duplicate `linux-mcp-server` tools
- Verify `go test -race ./...` passes
- Keep changes small and focused — one concern per PR
