> ⛔ Never open upstream PRs. Full rules: `cat ~/src/skills/workflow/SKILL.md`

# projectbluefin/bluefin-mcp

Semantic AI context layer for Bluefin — 11 read-only MCP tools, Go ports-and-adapters architecture.
Live binary: `~/.local/bin/bluefin-mcp` | Branch: `main`

## Skills

```bash
cat skills/SKILL.md                           # repo operational knowledge (architecture, tools, testdata)
cat ~/src/skills/bluefin-mcp/SKILL.md         # cross-cutting Bluefin MCP patterns
```

## Quick Start

```bash
just build          # go build -o bin/bluefin-mcp ./cmd/bluefin-mcp
just install        # install to ~/.local/bin/bluefin-mcp
just test           # go test -race ./...
just lint           # go vet ./... && staticcheck ./...
bash scripts/refresh-testdata.sh   # regenerate golden files from upstream Bluefin repos
```

## Critical Rules

- **Design law** — every tool must be grounded in `projectbluefin/common`, `ublue-os/bluefin`, or `docs.projectbluefin.io`; no generic Linux assumptions
- **Wayland only** — X11 is completely unsupported on Bluefin; never add X11 references
- **Read-only server** — no write operations except `store_unit_docs`; get explicit approval before adding another
- **No direct exec.Command** — all `internal/system/` functions must accept a `cli.CommandRunner` argument
- **No upstream data reporting** — tools surface data to the user only; nothing leaves the machine

## Work Queue

```bash
gh issue list --repo projectbluefin/bluefin-mcp --label copilot-ready --state open
```

## Session End

```bash
supermemory(mode="add", type="conversation", scope="project", content="[WHAT]...[WHY]...[FIX]...[NEXT]...")
```
