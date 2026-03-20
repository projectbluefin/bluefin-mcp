# bluefin-mcp

**MCP server providing AI context for Project Bluefin systems**
The opinionated companion to linux-mcp-server

![CI](https://github.com/projectbluefin/bluefin-mcp/actions/workflows/ci.yml/badge.svg)

---

## Why This Exists

Don't spend your time searching the internet trying to translate someone else's commands into your system setup.

The computer should know how it functions — so you can tell it what to do instead of having to remember commands. We know how to Unix. The goal is to automate the toil so you can work more efficiently.

`bluefin-mcp` gives your AI assistant the context it needs to do that: what the system is running, what the custom units do, which hardware works and which doesn't, and what automation is available. It doesn't guess. It reads the actual state of your actual machine.

> ⚠️ **This is an experiment.** We are figuring out whether this approach works. Under no circumstances should any tool in this server send automated reports, telemetry, or data to any upstream project, bug tracker, or external service. The server is read-only and local-only. If something is useful, a human decides what to do with it.

---

## What Is This?

`bluefin-mcp` is part of the **Bluefin Bluespeed** initiative — the project that brings AI-native tooling to [Project Bluefin](https://projectbluefin.io). We plan to use open models to help keep our computers in tip top shape by taking Linux troubleshooting to an entirely new level. That's the hope anyway, we're about to find out. If you want a piece of this action, now is the time to start contributing!

It works alongside [`linux-mcp-server`](https://github.com/redhat-et/linux-mcp-server), which handles raw system facts — journalctl output, systemd unit status, process lists, network state. `bluefin-mcp` is the **semantics layer**: it tells the AI what Bluefin-specific things actually *mean*.

For example, on Bluefin there is no `dnf`. Custom systemd units run opinionated automation on every boot. The desktop is not stock GNOME. Developer mode brings in tons of options, this context helps the agents give you better answers. Without `bluefin-mcp`, an AI assistant staring at a failed `flatpak-nuke-fedora.service` has no idea what it does or why it exists. With it, that knowledge is immediately available — this context keeps you safe!

The product is a "Troubleshooting" app that uses these two MCP servers with Goose as the UX, with local-first capabilities as well as the flexibility of using commercial LLMs. See the [Bluefin Documentation](https://docs.projectbluefin.io/troubleshooting)

Tired: Don't use those weird distros, you won't find help on the internet like you will with Ubuntu.
Wired: We have local LLMs that can respect our privacy and the source code to everything shipping on that image and the official docs. And that's it. It's clippy but real.

This is part of the reason Bluefin's cloud-native approach via `bootc` makes shipping this relatively straightforward. The MCP server will always be built to reflect what is actually on everyone's computer, making troubleshooting much more data driven instead of reddit driven.

---

## Division of Responsibility

| Layer | Server | Covers |
|---|---|---|
| **Facts** (what is happening) | `linux-mcp-server` | systemd status, journalctl, processes, logs, network, CPU, memory, disk, hardware inventory |
| **Semantics** (what Bluefin things mean) | `bluefin-mcp` | Custom unit docs, atomic OS state, ujust, Flatpak, Homebrew, Distrobox, hardware compatibility |

**Concrete example:** `linux-mcp-server` reports that `flatpak-nuke-fedora.service` failed. `bluefin-mcp` explains: that service removes Fedora's Flatpak remotes on every boot — Bluefin is Flathub-only. If it failed, the Fedora remote may still be active and producing duplicate app entries.

Install both for full coverage.

---

## What You Get — 16 User Scenarios

### 1. The Update Troubleshooter
You ran `ujust update` and it exited with an error. `check_updates` tells the AI whether an update is actually staged or if the local image is already current. `get_system_status` surfaces the booted digest and any staged update, letting the AI distinguish between a local conflict and an upstream availability issue before it suggests anything. Tracked in [issue #18](https://github.com/projectbluefin/bluefin-mcp/issues/18).

### 2. The Nvidia User
GPU performance degraded after an update. `get_variant_info` immediately confirms whether you're on the nvidia variant — a fact that determines which kernel parameters, drivers, and workaround services are in play. `get_system_status` shows the booted digest versus the staged image, so the AI can tell whether the regression came in with a recent update and whether rolling back is an option. Tracked in [issue #19](https://github.com/projectbluefin/bluefin-mcp/issues/19).

### 3. The Developer Setup
You switched to the DX image but `docker` commands still require `sudo`. `get_variant_info` confirms the DX variant is active. `get_unit_docs("bluefin-dx-groups.service")` explains that this service automatically adds every member of the `wheel` group to the `docker` and `incus-admin` groups on boot — so if it failed or you're not in `wheel`, sudo is still required until the next boot. Tracked in [issue #20](https://github.com/projectbluefin/bluefin-mcp/issues/20).

### 4. The Flatpak User
Expected apps are missing after a reinstall. `get_flatpak_list` shows what's actually installed and from which remote. The knowledge base explains that `flatpak-preinstall.service` — which installs the declarative Flatpak manifest — requires network connectivity on boot and silently does nothing if the network isn't up yet. Tracked in [issue #21](https://github.com/projectbluefin/bluefin-mcp/issues/21).

### 5. The Container Builder
Distrobox is failing to create or start containers. `list_distrobox` shows the current container inventory. `get_variant_info` confirms whether you're on a DX image, which ships the full container stack including Incus and libvirt alongside Distrobox — and whether the SELinux workaround services (`incus-workaround.service`, `libvirt-workaround.service`) are relevant. Tracked in [issue #22](https://github.com/projectbluefin/bluefin-mcp/issues/22).

### 6. The Recipe Hunter
You know Bluefin has a `ujust` command for something but can't remember the exact name. `list_recipes` returns the full current list of ujust recipes with their descriptions — including system maintenance tasks, developer tooling shortcuts, and hardware-specific utilities — exactly as they exist on your running system. Tracked in [issue #23](https://github.com/projectbluefin/bluefin-mcp/issues/23).

### 7. The New User
You're browsing `journalctl` and see `flatpak-nuke-fedora.service` complete on every boot and wonder if something is broken. `get_unit_docs("flatpak-nuke-fedora.service")` immediately returns a plain-English explanation: this service intentionally removes non-Flathub Flatpak remotes — it's working correctly, Bluefin enforces Flathub as the sole app source. Tracked in [issue #24](https://github.com/projectbluefin/bluefin-mcp/issues/24).

### 8. The VS Code User
You're not sure whether VS Code arrived as a Flatpak, a Homebrew package, or something baked into the image. `get_brew_packages` lists every Homebrew CLI package, and `get_flatpak_list` lists every installed Flatpak with its remote. Together they give the AI a definitive answer rather than a guess. Tracked in [issue #25](https://github.com/projectbluefin/bluefin-mcp/issues/25).

### 9. The Sysadmin
You want to audit what's been customized on a machine relative to the image defaults. `list_recipes` surfaces the full ujust surface, including `ujust check-local-overrides` — the built-in recipe that diffs your system's `/etc` against the image baseline and reports what has drifted. Tracked in [issue #26](https://github.com/projectbluefin/bluefin-mcp/issues/26).

### 10. The Laptop Evaluator
You're booted from a Bluefin LiveCD trying to figure out why WiFi isn't working before you commit to installing. `get_hardware_report` reads `lspci -nnk` and identifies a Broadcom WiFi adapter — a chip that requires a proprietary driver not included in the Fedora stock kernel. The AI tells you plainly: this adapter will not work on a standard Bluefin install. It also tells you what PCI ID was found (`14e4:43a0`) and that no kernel module was loaded. No internet search required; the system reported its own hardware. Instead we'll recommend you use Ubuntu, because this system is distribution agnostic, you just need someone to make an ubuntu-mcp server. It's only a matter of time. 

### 11. The DevContainers Developer
You open a repo in VS Code and select "Reopen in Container." Nothing happens — or Docker returns a permission error. `get_variant_info` immediately confirms whether you're on the DX image (devcontainers require it). `get_unit_docs("bluefin-dx-groups.service")` explains that this service adds `wheel` members to the `docker` group on boot — if you've never rebooted since enabling DX, the group assignment hasn't taken effect yet. `list_distrobox` shows your existing container inventory so the AI can distinguish a first-run setup problem from a broken re-open. Pairs naturally with the [`mcp-devcontainer`](https://mcpservers.org/servers/Siddhant-K-code/mcp-devcontainer) server (see [issue #4](https://github.com/projectbluefin/bluefin-mcp/issues/4)) for deeper devcontainer introspection.

### 12. The JetBrains Developer
JetBrains Toolbox is installed as a Flatpak. You've configured JetBrains Gateway to use Docker as a Dev Containers backend and IntelliJ reports "Docker not found." `get_flatpak_list` confirms Toolbox arrived as a Flatpak — and the AI can immediately explain the Flatpak sandbox boundary: Flatpak apps run in a confined namespace and do not inherit the Docker socket or the `docker` group membership that `bluefin-dx-groups.service` grants to your login session. `get_unit_docs("bluefin-dx-groups.service")` surfaces this context. `list_recipes` shows whether a `ujust` recipe exists to expose the socket to Flatpak apps. No 3-year-old forum post required. Tracked in [issue #15](https://github.com/projectbluefin/bluefin-mcp/issues/15).

### 13. The Rootless Podman Expert
You want to run a PostgreSQL + Redis stack using rootless Podman with Quadlet unit files so it starts automatically at login. `get_variant_info` confirms you're on DX, which ships Podman pre-configured for rootless use. `list_recipes` surfaces available `ujust` Podman helpers. `get_brew_packages` shows whether `podman-compose` is installed via Homebrew. `get_unit_docs` explains the DX service units that configure the Podman socket. The AI has full system context — which variant, which tools, which units are active — and can generate the correct Quadlet unit file for this exact machine rather than a generic template. Community Podman skills (see below) extend this expertise further. Tracked in [issue #14](https://github.com/projectbluefin/bluefin-mcp/issues/14).

### 14. The Home Lab Builder
You're running Incus VMs alongside Distrobox containers on a DX machine. A new Incus VM fails to start with a vague SELinux denial. `list_distrobox` shows your container inventory and confirms Distrobox is working. `get_unit_docs("incus-workaround.service")` explains that this service applies a targeted SELinux policy workaround required for Incus on Fedora-based systems — and what to check if it didn't run. `get_unit_docs("libvirt-workaround.service")` surfaces the companion libvirt workaround. `get_system_status` provides the booted image digest — useful if you're filing a bug report. Knowledge store improvements tracked in [issue #16](https://github.com/projectbluefin/bluefin-mcp/issues/16).

### 15. The Remote Development User
You SSH into a Bluefin DX machine to do remote development with VS Code Remote SSH or JetBrains Remote Dev. The remote process can't find `docker`, `brew`, or any Homebrew tools in PATH. `get_brew_packages` confirms the packages are installed. `get_unit_docs("ublue-user-setup.service")` explains that PATH is extended for interactive login shells by the user setup service — remote SSH connections may not source the full login environment unless explicitly configured. `list_recipes` shows whether a `ujust` recipe handles SSH remote development setup. The AI diagnoses the environment mismatch without guessing. Tracked in [issue #17](https://github.com/projectbluefin/bluefin-mcp/issues/17).

### 16. The Community Skills User
You ask: "Set up a Podman pod with PostgreSQL and Redis that starts at login." This is complex enough that even experienced Bluefin DX users may not know the Quadlet pattern. The AI loads a community-contributed `podman-quadlet` skill that provides expert guidance on Quadlet unit files. `bluefin-mcp` supplies the system context — DX variant confirmed, no existing postgres containers in `list_distrobox`, Podman socket active. The community skill provides the expertise; `bluefin-mcp` makes it specific to your actual system. The result: a working Quadlet config for this machine, not a generic template. This is the "everyone can be a Podman expert" story — see [Community Skills](#community-skills) below. Community skills system tracked in [issue #12](https://github.com/projectbluefin/bluefin-mcp/issues/12); tutorial skills backlog in [issue #13](https://github.com/projectbluefin/bluefin-mcp/issues/13).

---

## The 12 Tools

### Atomic OS State

| Tool | What it does |
|---|---|
| `get_system_status` | Returns the booted OCI image reference, digest, staged update (if any), and detected variant. The AI's first call when anything system-level goes wrong. |
| `check_updates` | Non-blocking check: is a newer Bluefin image available? Distinguishes "no update available" from "update staged, reboot required." |
| `get_boot_health` | Reports whether a rollback image is available and what it points to. Useful before and after any update. |
| `get_variant_info` | Identifies which Bluefin variant is running — `base`, `dx`, `nvidia`, `aurora`, or `aurora-dx`. Many issues are variant-specific. |

### Bluefin Automation

| Tool | What it does |
|---|---|
| `list_recipes` | Lists all `ujust` recipes available on the running system with descriptions. Bluefin's primary user automation surface. |

### Package Management

| Tool | What it does |
|---|---|
| `get_flatpak_list` | Lists every installed Flatpak application and its remote. Bluefin is Flathub-only; this confirms that's actually the case. |
| `get_brew_packages` | Lists all Homebrew CLI packages installed under Linuxbrew. |
| `list_distrobox` | Lists active Distrobox development containers with their images and status. |

### Hardware

| Tool | What it does |
|---|---|
| `get_hardware_report` | Runs `lspci -nnk` to get PCI devices with numeric vendor:device IDs and loaded kernel modules, reads `/sys/class/dmi/id/` for laptop model/chassis/firmware (no root required), detects LiveCD boot from `/proc/cmdline`, and checks whether the running Bluefin variant matches the detected GPU. Flags Broadcom WiFi (vendor `14e4`) and Nvidia GPUs on non-nvidia variants. The vendor list is a static embedded table — not a live database. For CPU, memory, disk, and full hardware inventory use `linux-mcp-server` instead. |

### Knowledge Store

| Tool | What it does |
|---|---|
| `get_unit_docs` | Fetches semantic documentation for a named Bluefin custom systemd unit — what it does, what variant it applies to, and what to check if it fails. |
| `store_unit_docs` | Store documentation for any custom unit. Persisted to `~/.local/share/bluefin-mcp`. |
| `list_unit_docs` | Lists every unit currently in the knowledge store. |

> **Ships pre-populated.** The knowledge store includes documentation for all 10 Bluefin custom systemd units out of the box: `ublue-system-setup.service`, `ublue-user-setup.service`, `flatpak-preinstall.service`, `flatpak-nuke-fedora.service`, `dconf-update.service`, `bazaar.service`, `bluefin-dx-groups.service`, `incus-workaround.service`, `libvirt-workaround.service`, and `swtpm-workaround.service`.

---

## Community Skills

`bluefin-mcp` provides **system context** — what's running, what variant, what's installed. Community skills provide **domain expertise** — how to do complex things correctly on Bluefin. Together, they make expert-level guidance available to everyone.

### How It Works

A community skill is a structured `SKILL.md` file that an AI agent loads on demand. When a user asks a complex question, the agent loads the relevant skill (expertise) and calls `bluefin-mcp` (system context) at the same time:

```
User: "Set up a PostgreSQL pod that starts at login"
AI loads: podman-quadlet skill (expertise)
AI calls: bluefin-mcp get_variant_info, list_distrobox (system state)
Result: Working Quadlet config tailored to this specific machine
```

The skill knows *how*. The MCP server knows *what's here*. Neither is sufficient alone.

### Planned Community Skills

| Skill | What It Teaches |
|---|---|
| `podman-quadlet` | Quadlet unit files for automatic container startup, volume management, networking |
| `podman-networking` | Rootless Podman networking — pasta, bridge networks, DNS, port forwarding |
| `podman-registry` | Private registry setup — `registries.conf`, mirror caching, policy |
| `distrobox-advanced` | Host binary export, GUI app integration, shared SSH agent, nested containers |
| `devcontainers-bluefin` | VS Code + JetBrains devcontainers on DX — docker socket, Flatpak boundary, devcontainer.json |
| `incus-intro` | First VM, networking, snapshots with Incus on DX |

Community skills will live in [`projectbluefin/community-skills`](https://github.com/projectbluefin/community-skills) (proposed). The format is identical to the existing skills in the Copilot CLI ecosystem — if you've written a skill before, you can contribute one.

> 💡 **The goal:** Everyone who runs Bluefin should be able to be a Podman expert, a container expert, an Incus expert — not because they spent months learning it, but because the system knows how to guide them through it, on their actual machine, with their actual setup.

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
- **Any MCP-compatible client** — [Goose](https://github.com/block/goose), Claude Desktop, or any other client that speaks the Model Context Protocol.

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
