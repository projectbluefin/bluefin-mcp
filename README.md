# bluefin-mcp

**MCP server providing AI context for Project Bluefin systems**

![CI](https://github.com/projectbluefin/bluefin-mcp/actions/workflows/ci.yml/badge.svg)

---

## What Is This?

`bluefin-mcp` is part of the **Bluefin Bluespeed** initiative — the project that brings AI-native tooling to [Project Bluefin](https://projectbluefin.io). We plan to use open models to help keep our computers in tip top shape by taking Linux troubleshooting to an entirely new level. That's the hope anyway, we're about to find out. If you want a piece of this action, now is the time to start contributing!

It works alongside [`linux-mcp-server`](https://github.com/redhat-et/linux-mcp-server), which handles raw system facts — journalctl output, systemd unit status, process lists, network state. `bluefin-mcp` is the **semantics layer**: it tells the AI what Bluefin-specific things actually *mean*.

For example, on Bluefin there is no `dnf`. Custom systemd units run opinionated automation on every boot. The desktop is not stock GNOME. Developer mode brings in tons of options, this context helps the agents give you better answers. Without `bluefin-mcp`, an AI assistant staring at a failed `flatpak-nuke-fedora.service` has no idea what it does or why it exists. With it, that knowledge is immediately available - this context keeps you safe! 

The product is a "Troubleshooting" app that are these two MCP servers using Goose as the UX, with local-first capabilities as well as the flexibility of using commercial LLMs. See the [Bluefin Documentation](https://docs.projectbluefin.io/troubleshooting)

Tired: Don't use those weird distros, you won't find help on the internet like you will with Ubuntu.
Wired: We have local LLMs that can respect our privacy and the source code to everything shipping on that image and the official docs. And that's it. It's clippy but real.

This is part of the reason Bluefin's cloud-native approach via `bootc` makes shipping this relatively straightforward. The MCP server will always be built to reflect of what is actually on everyone's computer, making troubleshooting much more data driven instead of reddit driven. 

---

## Division of Responsibility

| Layer | Server | Covers |
|---|---|---|
| **Facts** (what is happening) | `linux-mcp-server` | systemd status, journalctl, processes, logs, network |
| **Semantics** (what Bluefin things mean) | `bluefin-mcp` | Custom unit docs, atomic OS state, ujust, Flatpak, Homebrew, Distrobox |

**Concrete example:** `linux-mcp-server` reports that `flatpak-nuke-fedora.service` failed. `bluefin-mcp` explains: that service removes Fedora's Flatpak remotes on every boot — Bluefin is Flathub-only. If it failed, the Fedora remote may still be active and producing duplicate app entries.

Install both for full coverage.

---

## What You Get — What is this good for? 9 User Scenarios

### 1. The Update Troubleshooter
You ran `ujust update` and it exited with an error. `check_updates` tells the AI whether an update is actually staged or if the local image is already current. `get_system_status` surfaces the booted digest and any staged update, letting the AI distinguish between a local conflict and an upstream availability issue before it suggests anything.

### 2. The Nvidia User
GPU performance degraded after an update. `get_variant_info` immediately confirms whether you're on the nvidia variant — a fact that determines which kernel parameters, drivers, and workaround services are in play. `get_system_status` shows the booted digest versus the staged image, so the AI can tell whether the regression came in with a recent update and whether rolling back is an option.

### 3. The Developer Setup
You switched to the DX image but `docker` commands still require `sudo`. `get_variant_info` confirms the DX variant is active. `get_unit_docs("bluefin-dx-groups.service")` explains that this service automatically adds every member of the `wheel` group to the `docker` and `incus-admin` groups on boot — so if it failed or you're not in `wheel`, sudo is still required until the next boot.

### 4. The Flatpak User
Expected apps are missing after a reinstall. `get_flatpak_list` shows what's actually installed and from which remote. The knowledge base explains that `flatpak-preinstall.service` — which installs the declarative Flatpak manifest — requires network connectivity on boot and silently does nothing if the network isn't up yet.

### 5. The Container Builder
Distrobox is failing to create or start containers. `list_distrobox` shows the current container inventory. `get_variant_info` confirms whether you're on a DX image, which ships the full container stack including Incus and libvirt alongside Distrobox — and whether the SELinux workaround services (`incus-workaround.service`, `libvirt-workaround.service`) are relevant.

### 6. The Recipe Hunter
You know Bluefin has a `ujust` command for something but can't remember the exact name. `list_recipes` returns the full current list of ujust recipes with their descriptions — including system maintenance tasks, developer tooling shortcuts, and hardware-specific utilities — exactly as they exist on your running system.

### 7. The New User
You're browsing `journalctl` and see `flatpak-nuke-fedora.service` complete on every boot and wonder if something is broken. `get_unit_docs("flatpak-nuke-fedora.service")` immediately returns a plain-English explanation: this service intentionally removes non-Flathub Flatpak remotes — it's working correctly, Bluefin enforces Flathub as the sole app source.

### 8. The VS Code User
You're not sure whether VS Code arrived as a Flatpak, a Homebrew package, or something baked into the image. `get_brew_packages` lists every Homebrew CLI package, and `get_flatpak_list` lists every installed Flatpak with its remote. Together they give the AI a definitive answer rather than a guess.

### 9. The Sysadmin
You want to audit what's been customized on a machine relative to the image defaults. `list_recipes` surfaces the full ujust surface, including `ujust check-local-overrides` — the built-in recipe that diffs your system's `/etc` against the image baseline and reports what has drifted.

---

## The 11 Tools

### Atomic OS State

| Tool | What it does for you |
|---|---|
| `get_system_status` | Returns the booted OCI image reference, digest, staged update (if any), and detected variant. The AI's first call when anything system-level goes wrong. |
| `check_updates` | Non-blocking check: is a newer Bluefin image available? Distinguishes "no update available" from "update staged, reboot required." |
| `get_boot_health` | Reports whether a rollback image is available and what it points to. Useful before and after any update. |
| `get_variant_info` | Identifies which Bluefin variant is running — `base`, `dx`, `nvidia`, `aurora`, or `aurora-dx`. Many issues are variant-specific. |

### Bluefin Automation

| Tool | What it does for you |
|---|---|
| `list_recipes` | Lists all `ujust` recipes available on the running system with descriptions. Bluefin's primary user automation surface. |

### Package Management

| Tool | What it does for you |
|---|---|
| `get_flatpak_list` | Lists every installed Flatpak application and its remote. Bluefin is Flathub-only; this confirms that's actually the case. |
| `get_brew_packages` | Lists all Homebrew CLI packages installed under Linuxbrew. |
| `list_distrobox` | Lists active Distrobox development containers with their images and status. |

### Knowledge Store

| Tool | What it does for you |
|---|---|
| `get_unit_docs` | Fetches semantic documentation for a named Bluefin custom systemd unit — what it does, what variant it applies to, and what to check if it fails. |
| `store_unit_docs` | Lets you (or your AI) add documentation for any additional custom unit. Persisted to `~/.local/share/bluefin-mcp`. |
| `list_unit_docs` | Lists every unit currently in the knowledge store. |

> **Ships pre-populated.** The knowledge store includes documentation for all 10 Bluefin custom systemd units out of the box: `ublue-system-setup.service`, `ublue-user-setup.service`, `flatpak-preinstall.service`, `flatpak-nuke-fedora.service`, `dconf-update.service`, `bazaar.service`, `bluefin-dx-groups.service`, `incus-workaround.service`, `libvirt-workaround.service`, and `swtpm-workaround.service`.

---

## Installation

```bash
brew install ublue-os/tap/bluefin-mcp
```

Available via [`ublue-os/homebrew-tap`](https://github.com/ublue-os/homebrew-tap) — formula coming soon.

---

## MCP Client Configuration

Add this to your MCP client's server configuration:

```json
{
  "mcpServers": {
    "bluefin": {
      "command": "/home/linuxbrew/.linuxbrew/bin/bluefin-mcp"
    }
  }
}
```

> **⚠️ Use the absolute path.** MCP clients spawn the server as a subprocess and may not inherit your shell's `PATH`. A bare `bluefin-mcp` command will silently fail if the binary isn't on the client's inherited path.

On Bluefin, Homebrew installs to `/home/linuxbrew/.linuxbrew/`. Confirm the path after installation with:

```bash
which bluefin-mcp
```

---

## Works With

- **`linux-mcp-server`** (rhel-lightspeed) — the facts layer. Install both for full AI coverage of your Bluefin system: raw diagnostics from `linux-mcp-server`, Bluefin-specific semantics from `bluefin-mcp`.
- **Any MCP-compatible client** — Claude Desktop, [goose](https://github.com/block/goose), or any other client that speaks the Model Context Protocol.

---

## Building from Source

```bash
git clone https://github.com/projectbluefin/bluefin-mcp
cd bluefin-mcp
just build
just install   # installs to ~/.local/bin/bluefin-mcp
```

Other targets:

```bash
just test   # go test -race ./...
just lint   # go vet + staticcheck
just clean  # remove bin/
```
