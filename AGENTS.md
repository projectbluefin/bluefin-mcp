# AGENTS.md — bluefin-mcp

Maintenance guide for AI agents. **Read every section before touching any file.**

---

## ⚠️ Design Law — Read Before Changing Anything

Every tool, field, and default value in this codebase MUST be grounded in:

1. `projectbluefin/common` source code
2. `ublue-os/bluefin` source code
3. `docs.projectbluefin.io`

**Never design from generic Linux assumptions:**

- **Wayland only** — X11 is completely unsupported on Bluefin. Never add X11 detection, references, or fallbacks.
- **No dnf/rpm/apt** — the root filesystem is read-only. Package management = flatpak, brew, ujust only.
- **Fully read-only server** — bluefin-mcp has no write operations except `store_unit_docs`. Never add execute functionality.
- **Before adding any tool or field**: verify it exists in Bluefin source or official docs. If not there, it doesn't belong here.

The canonical skill file with full context: `~/src/skills/bluefin-mcp/SKILL.md`

---

## Architecture

bluefin-mcp follows the **Ports & Adapters** (hexagonal) pattern.

```
cmd/bluefin-mcp/main.go          Entry point. Wires everything together.
internal/cli/executor.go         Port: CommandRunner interface
internal/cli/real.go             Adapter: RealExecutor (exec.Command)
internal/cli/mock.go             Adapter: MockExecutor (test double)
internal/system/                 Domain: one file per subsystem
internal/tools/tools.go          Thin MCP glue — no business logic
internal/seed/                   Embedded seed data (go:embed)
testdata/                        Golden files from real Bluefin output
```

### CommandRunner interface

Defined in `internal/cli/executor.go`:

```go
type CommandRunner interface {
    Run(ctx context.Context, name string, args []string) ([]byte, error)
}
```

- `ErrNotInstalled` is returned when the binary is absent from PATH.
- **All `internal/system/` packages receive an injected `CommandRunner`** — they never call `exec.Command` directly.
- `RealExecutor` (`internal/cli/real.go`) runs actual system commands with process-group management so context cancellation can SIGTERM → SIGKILL the process tree.

### Tool handlers

`internal/tools/tools.go` — `Register()` wires all **11 tools** to the MCP server. Handlers are thin glue:

1. Unmarshal the request argument(s).
2. Call the appropriate `internal/system/` function.
3. Return `jsonResult(...)` or `mcp.NewToolResultError(...)`.

No business logic belongs here. All parsing, command invocation, and data transformation live in `internal/system/`.

### Current tool surface

| Tool | Description |
|---|---|
| `get_system_status` | Atomic OCI image state (image ref, digest, staged update, variant) |
| `check_updates` | Non-blocking update availability check |
| `get_boot_health` | Last boot health and rollback availability |
| `get_variant_info` | Detect variant: base, dx, nvidia, aurora, aurora-dx |
| `list_recipes` | Available `ujust` recipes |
| `get_flatpak_list` | Installed Flatpak applications |
| `get_brew_packages` | Installed Homebrew CLI packages |
| `list_distrobox` | Active Distrobox containers |
| `get_unit_docs` | Retrieve semantic docs for a Bluefin systemd unit |
| `store_unit_docs` | **Only write op** — persist unit documentation |
| `list_unit_docs` | All documented Bluefin systemd units |

### Knowledge store

`internal/system/knowledge.go` — `KnowledgeStore`:

- **Thread-safe**: all mutations hold `sync.Mutex`.
- **Atomic writes**: `os.WriteFile` to a `.tmp` file, then `os.Rename` — no partial writes visible to readers.
- **Seed data**: `internal/seed/units.json` is embedded at compile time via `//go:embed`. Loaded first; user additions in `~/.local/share/bluefin-mcp/units.json` overlay and override seed entries.
- **Unit name validation**: regex enforces valid systemd unit names before any write.

---

## stdout Rule

```
os.Stdout is redirected to os.Stderr in main() before ServeStdio().
Any fmt.Println or log.Println to stdout corrupts the MCP JSON-RPC protocol.
All logging must use fmt.Fprintf(os.Stderr, ...) or the stderr logger.
```

This assignment happens as the **very first statement** in `main()`:

```go
os.Stdout = os.Stderr
```

Never remove it, never move it later, never write to `os.Stdout` anywhere in the codebase.

---

## Adding a New Tool — Checklist

Before writing a single line of code, verify all of the following:

- [ ] **Grounded in Bluefin source**: the tool's subject exists in `projectbluefin/common`, `ublue-os/bluefin`, or `docs.projectbluefin.io`. Generic Linux facts belong in `linux-mcp-server`, not here.
- [ ] **Read-only**: no write or execute operations. The only exception in this entire codebase is `store_unit_docs`. Get explicit approval before adding another write.
- [ ] **Optional-binary graceful degradation**: if the tool calls `brew`, `distrobox`, or any other optional binary, it must return an empty list (not an error) when the binary is absent. `ErrNotInstalled` is already handled by `RealExecutor` — check for it in `internal/system/` and degrade cleanly.
- [ ] **No direct exec.Command**: the new `internal/system/` function must accept a `cli.CommandRunner` argument. Never import `os/exec` in `internal/system/`.
- [ ] **Tested with golden file**: add a `_test.go` in `internal/system/` that uses `MockExecutor` loaded from a real-Bluefin output file in `testdata/`. Do not invent test data.
- [ ] **Correct placement**: if the answer is a static Linux fact, it belongs in `linux-mcp-server`. If it requires Bluefin-specific semantics, it belongs here.
- [ ] **Handler is thin**: the tool handler in `internal/tools/tools.go` must contain no parsing, computation, or branching logic beyond argument extraction and result marshalling.

---

## Testdata

`testdata/` and `internal/seed/units.json` are **auto-generated** from the Bluefin source repos on GitHub — never hand-edited.

### Sources of truth

| Repo | What it provides |
|---|---|
| `projectbluefin/common` | ujust recipes (`.just` files), systemd units |
| `ublue-os/bluefin` | image refs, variant definitions, DX systemd units |
| `ublue-os/bluefin-lts` | LTS variant (pinned for SHA traceability) |

### How to refresh

```bash
bash scripts/refresh-testdata.sh   # requires gh CLI (authenticated)
go test -race ./...                # verify nothing broke
```

The script fetches the current HEAD of each repo, extracts recipes / unit files / flatpak lists, and writes fresh golden files. `testdata/refresh-manifest.json` records the exact commit SHAs used — check it to know what version generated the current files.

### Weekly automation

`.github/workflows/refresh-testdata.yml` runs every Sunday at 00:00 UTC (and on `workflow_dispatch`). It runs the script, verifies `go test -race ./...` passes, then commits any changes with `[skip ci]`.

### If tests break after a refresh

The upstream source format changed. Update the parser in `internal/system/` to handle the new format, then re-run `bash scripts/refresh-testdata.sh` to regenerate. Do **not** hand-edit testdata to make tests pass.

---

## Testing

```bash
go test -race ./...   # always run with race detector
```

- **Golden files** live in `testdata/` and are auto-generated by `scripts/refresh-testdata.sh` from the Bluefin source repos. Never hand-craft or invent testdata content.
- **`MockExecutor`** (`internal/cli/mock.go`) is the only approved test double for `CommandRunner`. Use `SetResponse` to load golden file content.
- **No real subprocess calls** in any test. Tests must pass on any machine, including CI containers without a Bluefin runtime.
- Each `internal/system/` file has a corresponding `_test.go`. New code without tests will not be merged.

---

## Build & Deploy

```bash
just build          # go build -o bin/bluefin-mcp ./cmd/bluefin-mcp
just install        # cp bin/bluefin-mcp ~/.local/bin/bluefin-mcp
just test           # go test -race ./...
just lint           # go vet ./... && staticcheck ./...
just verify-binary  # ~/.local/bin/bluefin-mcp --version
just clean          # rm -rf bin/
```

MCP client configuration **must use an absolute path** — never a bare binary name:

```json
{
  "mcpServers": {
    "bluefin": {
      "command": "/home/<user>/.local/bin/bluefin-mcp"
    }
  }
}
```

A bare binary name will fail because MCP clients typically do not inherit the user's full `PATH`.
