# AGENTS.md — bluefin-mcp

Maintenance guide for AI agents. **Read every section before touching any file.**

---

## ⛔ RULE #1 — NEVER open PRs to upstream repositories

Agents MUST NEVER open pull requests to repositories outside @castrojo's or projectbluefin's control.

**Upstream = any org not owned by @castrojo or projectbluefin:**
- ublue-os/* (including ublue-os/homebrew-tap) → UPSTREAM
- rhel-lightspeed/* → UPSTREAM
- homebrew/* → UPSTREAM
- Any org not explicitly listed as "own" below → UPSTREAM

**Own repos (agents MAY create branches + PRs):**
- castrojo/* (copilot-config, powerlevel, dotcopilot, etc.)
- projectbluefin/* (bluefin-mcp, etc.)

**Correct workflow for upstream contributions:**
1. Prepare the change locally (branch, commit to local clone)
2. File a tracking ISSUE in the **project's own repo** with: what to PR, where, exact PR content
3. Human reviews and opens the upstream PR manually — NEVER the agent

This rule was established after an agent automatically opened ublue-os/homebrew-tap#308 and had to close it immediately. No exceptions, no escalations, no "just this once."

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

## Project Philosophy — The Why

> Don't spend your time searching the internet trying to translate someone else's commands into your system setup. The computer should know how it functions — so you can tell it what to do instead of having to remember commands. We know how to Unix. Automate the toil so we can work more efficiently.

This mantra has concrete implications for every tool decision:

- **The computer knows its state.** Tools read real system data — `bootc status`, `lspci -nnk`, sysfs — not heuristics or guesses.
- **Defer to `linux-mcp-server` where possible.** That server handles generic Linux facts (CPU, memory, disk, journalctl, services). `bluefin-mcp` adds only the Bluefin-specific semantic layer on top. Never duplicate.
- **No internet required.** Every tool works entirely from local system state. The AI reasons; the server provides data. Nothing is fetched.
- **Honest output only.** Tool descriptions and README scenarios describe exactly what each tool reads and what it returns. No aspirational language, no implied capabilities that don't exist yet.
- **The user should not need to know commands.** If a user has to look up a command to understand what a tool returned, the tool description is failing its job.

---

## ⛔ No Automated Upstream Reporting — Ever

This project is an **experiment**. We are testing whether this approach works. Hard constraints:

- **Never add a tool that sends data anywhere** — no bug reports, no telemetry, no pings, no HTTP calls, no issue filing. The server is read-only and local-only. Period.
- **Never frame a tool as a data pipeline to upstream projects.** Tools surface information to the user and their AI. The human decides what to do with that information.
- **The only write operation permitted is `store_unit_docs`** — writing to the local knowledge store on the user's own machine. Nothing leaves the machine.

If you are an agent and you are considering adding a tool that contacts an external service, stop. That is not this project.

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

## Agentic Change Tracking

A weekly GitHub Actions workflow monitors upstream Bluefin source repos for changes and automatically generates detailed enhancement issues via the GitHub Models API.

### How It Works

1. **`scripts/track-bluefin-changes.py`** runs weekly (Sunday midnight UTC) via `.github/workflows/track-bluefin-changes.yml`
2. It fetches current state from `projectbluefin/common`, `ublue-os/bluefin`, and `ublue-os/bluefin-lts` — ujust recipes, systemd units, and image variants
3. Diffs against `tracking/bluefin-state.json` (committed baseline state from the previous run)
4. For each new/removed item, calls **GitHub Models API (gpt-4o)** to generate a richly detailed issue body with implementation plan, test requirements, and TDD checklist
5. Files the issue in `projectbluefin/bluefin-mcp` with the full label taxonomy
6. Commits the updated `tracking/bluefin-state.json` back to `main` with `[skip ci]`

This is the **self-improving loop**:
> Bluefin ships feature → GHA detects → AI generates detailed issue → human approves → agent implements with TDD → repeat

### Label Taxonomy (key labels for agents)

| Label | Meaning |
|---|---|
| `auto-generated` | Created by the weekly tracker — do not edit manually |
| `needs-human-review` | **DO NOT implement** — awaiting human approval |
| `approved` | Human approved — ready for agent implementation |
| `tdd-ready` | Issue has a full TDD checklist an agent can follow |
| `tier:ujust` | Relates to ujust recipe tooling |
| `tier:knowledge-store` | Relates to `internal/seed/units.json` |
| `source:common` | Change originated in `projectbluefin/common` |
| `source:bluefin` | Change originated in `ublue-os/bluefin` |
| `variant:dx` / `variant:lts` / `variant:all` | Which Bluefin variant is affected |

### Agent Implementation Rules

When picking up an `approved` + `tdd-ready` issue:

1. **Read `AGENTS.md` first** — especially the Design Law section above.
2. **Follow the TDD checklist in the issue** exactly: write failing test → implement → verify green → refactor.
3. Run `go test -race ./...` — all tests must pass before opening a PR.
4. Verify design law compliance: Wayland-only, no dnf/rpm, read-only server, grounded in Bluefin source.
5. **Never implement a `needs-human-review` issue** without the `approved` label.

### Bootstrap (one-time setup)

```bash
# Create all labels in the repo
bash scripts/setup-labels.sh

# Capture baseline state (first run — no issues filed)
GITHUB_TOKEN=$(gh auth token) python3 scripts/track-bluefin-changes.py
```

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
