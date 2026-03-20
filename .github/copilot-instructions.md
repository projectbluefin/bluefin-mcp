# bluefin-mcp — Copilot Instructions

## Build & Test

```bash
just build          # go build -o bin/bluefin-mcp ./cmd/bluefin-mcp
just test           # go test -race ./...
just lint           # go vet ./... && staticcheck ./...
just install        # build + cp to ~/.local/bin/bluefin-mcp
just verify-binary  # smoke test: --version handshake
```

Run a single test file: `go test -race ./internal/system/ -run TestGetRecipes`

CI gate: `go test -race ./...` + `go vet ./...` + `staticcheck ./...` must all pass before any task is declared done.

## What This Server Does (and Does NOT Do)

bluefin-mcp is the **semantic layer** for Bluefin AI assistance. It answers "what does this Bluefin-specific thing mean?" — not "what is happening on this system right now?"

| Owned by bluefin-mcp | Owned by linux-mcp-server (Red Hat) |
|---|---|
| Bluefin custom unit descriptions | systemd unit status, logs, journal |
| ujust recipe inventory | Process list, network, storage |
| Atomic OS state (bootc/OCI) | CPU, memory, hardware info |
| Flatpak/brew package lists | Generic Linux diagnostics |
| Distrobox container list | — |

**Never add tools that duplicate linux-mcp-server.** Before designing any new tool, verify it belongs to the Bluefin semantic layer, not the Linux facts layer.

## Bluefin Platform Constraints — Non-Negotiable

Bluefin is **not a traditional Linux desktop**. These constraints are absolute:

- **Wayland only** — X11 is completely unsupported. Never add X11 detection, references, or fallbacks.
- **Read-only root filesystem** — no `dnf`, `rpm`, `apt`, or any system package manager. Package management is `flatpak`, `brew` (Linuxbrew at `/home/linuxbrew/.linuxbrew`), and `ujust` only.
- **Atomic OCI image** — the OS is an immutable container image. Updates are bootc image pulls, not package upgrades.
- **Fully read-only MCP server** — the only write operation permitted is `store_unit_docs`. Never add execute functionality.

Before adding any tool or field: verify it exists in `projectbluefin/common` source, `ublue-os/bluefin` source, or `docs.projectbluefin.io`. If not found there, it does not belong here.

## MCP Protocol — stdout Is Sacred

**The first line of `cmd/bluefin-mcp/main.go` must redirect stdout before anything else:**

```go
os.Stdout = os.Stderr  // must be first — any stdout write corrupts JSON-RPC
```

Any `fmt.Println`, `log.Println`, or debug output to stdout will silently corrupt the MCP JSON-RPC stream. CI enforces this with a grep check. Never remove the redirect. Never add stdout writes.

## Testdata — Source-Driven Only

`testdata/` and `internal/seed/units.json` are populated from real Bluefin source repos via `scripts/refresh-testdata.sh`. **Never hand-edit these files.**

```bash
# To update testdata:
bash scripts/refresh-testdata.sh   # fetches from projectbluefin/common + ublue-os/bluefin
```

`testdata/refresh-manifest.json` records the source SHAs for traceability. The weekly GHA (`refresh-testdata.yml`) runs this automatically. Invented testdata is a bug.

## Upstream Contributions — Manual Only (Rule #1)

Agents MUST NOT open PRs to `ublue-os/*`, `rhel-lightspeed/*`, or any org outside `castrojo/*` / `projectbluefin/*`.

For upstream contributions (e.g., `ublue-os/homebrew-tap` formula):
1. Prepare the change locally
2. File a tracking issue in **this repo** with the exact PR content
3. Human opens the upstream PR manually
