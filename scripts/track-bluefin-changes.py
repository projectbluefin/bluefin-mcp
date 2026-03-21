#!/usr/bin/env python3
"""Agentic Bluefin change tracker.

Weekly: fetches projectbluefin/common, ublue-os/bluefin, ublue-os/bluefin-lts,
diffs vs last known state, uses GitHub Models API to generate rich MCP enhancement
issues, files them with labels, updates tracking state.
"""

import json
import os
import re
import subprocess
import sys
import urllib.error
import urllib.request
from datetime import datetime, timezone
from pathlib import Path

REPO_ROOT = Path(__file__).parent.parent
TRACKING_FILE = REPO_ROOT / "tracking" / "bluefin-state.json"
MCP_REPO = "projectbluefin/bluefin-mcp"
GITHUB_TOKEN = os.environ["GITHUB_TOKEN"]

SOURCES = {
    "common": "projectbluefin/common",
    "bluefin": "ublue-os/bluefin",
    "bluefin-lts": "ublue-os/bluefin-lts",
}

JUST_PATHS = {
    "common": [
        "system_files/shared/usr/share/ublue-os/just",
        "system_files/bluefin/usr/share/ublue-os/just",
    ],
}

UNIT_PATHS = {
    "common": [
        "system_files/shared/usr/lib/systemd/system",
        "system_files/shared/usr/lib/systemd/user",
        "system_files/bluefin/usr/lib/systemd/system",
    ],
    "bluefin": [
        "system_files/shared/usr/lib/systemd/system",
        "system_files/dx/usr/lib/systemd/system",
    ],
    "bluefin-lts": [
        "system_files/shared/usr/lib/systemd/system",
    ],
}


def gh_api(path, **kwargs):
    """Call GitHub API via gh CLI."""
    cmd = ["gh", "api", path, "--paginate"]
    result = subprocess.run(cmd, capture_output=True, text=True, check=True)
    return json.loads(result.stdout)


def get_repo_sha(repo):
    """Get latest commit SHA on default branch."""
    data = gh_api(f"repos/{repo}/commits/HEAD")
    return data["sha"]


def list_directory(repo, path):
    """List files in a GitHub repo directory."""
    try:
        data = gh_api(f"repos/{repo}/contents/{path}")
        if isinstance(data, list):
            return data
        return []
    except subprocess.CalledProcessError:
        return []


def fetch_file(download_url):
    """Fetch raw file content from GitHub."""
    req = urllib.request.Request(
        download_url,
        headers={"Authorization": f"Bearer {GITHUB_TOKEN}"},
    )
    with urllib.request.urlopen(req) as r:
        return r.read().decode("utf-8")


def parse_just_recipes(content):
    """Extract recipe name + description from a .just file."""
    recipes = {}
    lines = content.splitlines()
    for i, line in enumerate(lines):
        # Recipe definition: starts with identifier followed by colon (no leading whitespace)
        m = re.match(r'^([a-zA-Z][a-zA-Z0-9_-]*):', line)
        if m:
            name = m.group(1)
            # Look for # comment on same line or previous line
            desc = ""
            if "#" in line:
                desc = line.split("#", 1)[1].strip()
            elif i > 0 and lines[i-1].strip().startswith("#"):
                desc = lines[i-1].strip().lstrip("# ").strip()
            recipes[name] = desc
    return recipes


def parse_unit_description(content):
    """Extract Description= from a systemd unit file."""
    for line in content.splitlines():
        if line.startswith("Description="):
            return line.split("=", 1)[1].strip()
    return ""


def fetch_current_state():
    """Fetch current recipes and units from all source repos."""
    state = {
        "fetched_at": datetime.now(timezone.utc).isoformat(),
        "shas": {},
        "recipes": {},
        "units": {},
    }

    # Get SHAs
    for key, repo in SOURCES.items():
        try:
            state["shas"][key] = get_repo_sha(repo)
        except Exception as e:
            print(f"  Warning: could not get SHA for {repo}: {e}")
            state["shas"][key] = "unknown"

    # Fetch recipes from common
    repo = SOURCES["common"]
    for just_path in JUST_PATHS.get("common", []):
        files = list_directory(repo, just_path)
        for f in files:
            if f["name"].endswith(".just") and f.get("download_url"):
                try:
                    content = fetch_file(f["download_url"])
                    recipes = parse_just_recipes(content)
                    for name, desc in recipes.items():
                        state["recipes"][name] = {
                            "description": desc,
                            "source_file": f["path"],
                            "source_repo": repo,
                        }
                except Exception as e:
                    print(f"  Warning: could not fetch {f['path']}: {e}")

    # Fetch units from common and bluefin-lts
    for source_key, paths in UNIT_PATHS.items():
        repo = SOURCES[source_key]
        variant = "lts" if source_key == "bluefin-lts" else "all"
        for unit_path in paths:
            files = list_directory(repo, unit_path)
            for f in files:
                if f["name"].endswith((".service", ".timer", ".socket", ".target")) and f.get("download_url"):
                    try:
                        content = fetch_file(f["download_url"])
                        desc = parse_unit_description(content)
                        # Detect DX variant from path
                        unit_variant = variant
                        if "bluefin" in f["path"] and "dx" in f["path"].lower():
                            unit_variant = "dx"
                        state["units"][f["name"]] = {
                            "description": desc,
                            "variant": unit_variant,
                            "source_file": f["path"],
                            "source_repo": repo,
                        }
                    except Exception as e:
                        print(f"  Warning: could not fetch {f['path']}: {e}")

    return state


def diff_states(old, new):
    """Return lists of added/removed/changed items."""
    changes = []

    # New or changed recipes
    for name, info in new["recipes"].items():
        if name not in old.get("recipes", {}):
            changes.append({
                "type": "new_recipe",
                "name": name,
                "info": info,
                "source_repo": info["source_repo"],
            })

    # Removed recipes
    for name in old.get("recipes", {}):
        if name not in new["recipes"]:
            changes.append({
                "type": "removed_recipe",
                "name": name,
                "info": old["recipes"][name],
            })

    # New or changed units
    for name, info in new["units"].items():
        if name not in old.get("units", {}):
            changes.append({
                "type": "new_unit",
                "name": name,
                "info": info,
                "source_repo": info["source_repo"],
            })

    return changes


def call_github_models(prompt):
    """Call GitHub Models API (gpt-4o) to generate issue content."""
    url = "https://models.inference.ai.azure.com/chat/completions"
    payload = json.dumps({
        "model": "gpt-4o",
        "messages": [
            {
                "role": "system",
                "content": (
                    "You are an expert Go developer designing MCP (Model Context Protocol) tools "
                    "for bluefin-mcp, a read-only Go MCP server that provides AI context for "
                    "Project Bluefin — a Wayland-only atomic OCI-based Linux desktop. "
                    "Bluefin is NOT traditional Linux: no X11, no dnf/rpm, root FS is read-only, "
                    "packages = flatpak/brew/ujust only. Source of truth = projectbluefin/common "
                    "and ublue-os/bluefin source code. "
                    "The server uses Ports & Adapters (CommandRunner interface), all tools are read-only, "
                    "tests use MockExecutor with golden testdata files. "
                    "linux-mcp-server (Red Hat) owns: systemd facts, journalctl, processes, logs. "
                    "bluefin-mcp owns: semantics, Bluefin-specific surfaces."
                ),
            },
            {"role": "user", "content": prompt},
        ],
        "max_tokens": 2000,
        "temperature": 0.3,
    }).encode()

    req = urllib.request.Request(
        url,
        data=payload,
        headers={
            "Authorization": f"Bearer {GITHUB_TOKEN}",
            "Content-Type": "application/json",
        },
    )
    try:
        with urllib.request.urlopen(req, timeout=30) as r:
            data = json.loads(r.read())
            return data["choices"][0]["message"]["content"]
    except Exception as e:
        print(f"  Warning: GitHub Models API call failed: {e}")
        return None


def generate_issue_for_change(change):
    """Use GitHub Models to generate rich issue content for a detected change."""
    if change["type"] == "new_recipe":
        prompt = f"""A new ujust recipe was added to Bluefin:

Recipe name: `ujust {change['name']}`
Description from source: "{change['info']['description']}"
Source file: {change['info']['source_file']}
Source repo: {change['info']['source_repo']}

Generate a detailed GitHub issue for the bluefin-mcp repository proposing how to expose or document this recipe via the MCP server. The issue should include:

1. **Summary** (2-3 sentences on what this recipe does and why it matters to Bluefin users)
2. **Proposed MCP Enhancement** (should this be a new tool? An addition to list_recipes output? A knowledge store entry? Remember: server is read-only, no execute operations)
3. **Implementation Plan** (specific Go code guidance: which file in internal/system/, what the function signature should look like, how it fits the CommandRunner pattern)
4. **Test Requirements** (what golden testdata file to add, what test cases to write in _test.go, covering both happy path and edge cases)
5. **TDD Checklist** (checkbox list an agent can follow: write failing test → implement → verify green → refactor)
6. **Design Law Reminder** (one-line confirmation this doesn't violate: Wayland-only, no dnf, read-only server, grounded in Bluefin source)

Format as a GitHub issue body in Markdown. Be specific and actionable."""

    elif change["type"] == "new_unit":
        prompt = f"""A new systemd unit was found in Bluefin source:

Unit name: `{change['name']}`
Description from unit file: "{change['info']['description']}"
Variant: {change['info']['variant']}
Source file: {change['info']['source_file']}
Source repo: {change['info']['source_repo']}

Generate a detailed GitHub issue for the bluefin-mcp repository. This unit should be added to the pre-populated knowledge store (internal/seed/units.json) so AI agents can explain what it does to Bluefin users. The issue should include:

1. **Summary** (what this unit does in the Bluefin context, why a user or agent would need to know about it)
2. **Proposed Knowledge Base Entry** (the exact JSON to add to internal/seed/units.json, with name, variant, and a thorough description that explains: what it runs, when it runs, why it exists on Bluefin specifically, and what to check if it fails)
3. **Test Requirements** (add test to knowledge_test.go: TestKnowledge_SeedContains_{unit_stem}_Unit that verifies the entry is present in the seed)
4. **TDD Checklist** (checkbox list for implementing agent)
5. **Design Law Reminder**

Format as a GitHub issue body in Markdown."""

    elif change["type"] == "removed_recipe":
        prompt = f"""A ujust recipe was REMOVED from Bluefin:

Recipe name: `ujust {change['name']}`
Was in: {change['info']['source_file']}

Generate a brief GitHub issue noting this removal and asking:
1. Should list_recipes output be updated to not reference this recipe?
2. Are there any hardcoded references to this recipe in the MCP server that need removing?
3. Does the knowledge store have an entry for this that should be cleaned up?

Format as a concise GitHub issue body."""

    else:
        return None

    return call_github_models(prompt)


def determine_labels(change):
    """Return appropriate label list for a change."""
    labels = ["feature-idea", "auto-generated", "needs-human-review", "tdd-ready"]

    if change["type"] in ("new_recipe", "removed_recipe"):
        labels.append("tier:ujust")
    elif change["type"] == "new_unit":
        labels.append("tier:knowledge-store")

    info = change.get("info", {})
    variant = info.get("variant", "all")
    if variant == "dx":
        labels.append("variant:dx")
    elif variant == "lts":
        labels.append("variant:lts")
    elif variant == "all":
        labels.append("variant:all")

    source_repo = info.get("source_repo", "")
    if "common" in source_repo:
        labels.append("source:common")
    elif "bluefin-lts" in source_repo:
        labels.append("source:bluefin-lts")
    elif "bluefin" in source_repo:
        labels.append("source:bluefin")

    return labels


def file_issue(title, body, labels):
    """File a GitHub issue in the MCP repo."""
    label_args = []
    for lb in labels:
        label_args += ["--label", lb]

    cmd = [
        "gh", "issue", "create",
        "--repo", MCP_REPO,
        "--title", title,
        "--body", body,
    ] + label_args

    result = subprocess.run(cmd, capture_output=True, text=True, check=True)
    return result.stdout.strip()


def main():
    TRACKING_FILE.parent.mkdir(exist_ok=True)

    # Load previous state
    old_state = {}
    if TRACKING_FILE.exists():
        raw = TRACKING_FILE.read_text().strip()
        if raw and raw != "{}":
            old_state = json.loads(raw)

    print("==> Fetching current Bluefin source state...")
    new_state = fetch_current_state()
    print(f"    Recipes found: {len(new_state['recipes'])}")
    print(f"    Units found:   {len(new_state['units'])}")

    # First run: just save state, don't file issues for everything
    if not old_state:
        print("==> First run — saving baseline state (no issues filed)")
        TRACKING_FILE.write_text(json.dumps(new_state, indent=2))
        print(f"    Baseline saved to {TRACKING_FILE}")
        return

    print("==> Diffing vs previous state...")
    changes = diff_states(old_state, new_state)
    print(f"    Changes detected: {len(changes)}")

    issues_filed = []
    for change in changes:
        print(f"  -> {change['type']}: {change['name']}")
        body = generate_issue_for_change(change)
        if not body:
            print(f"     Skipping (no issue body generated)")
            continue

        if change["type"] == "new_recipe":
            title = f"feat: expose new ujust recipe `{change['name']}` via MCP"
        elif change["type"] == "new_unit":
            title = f"feat: add `{change['name']}` to pre-populated knowledge store"
        elif change["type"] == "removed_recipe":
            title = f"chore: ujust recipe `{change['name']}` removed from Bluefin"
        else:
            continue

        labels = determine_labels(change)
        try:
            url = file_issue(title, body, labels)
            print(f"     Filed: {url}")
            issues_filed.append({"change": change["name"], "url": url})
        except subprocess.CalledProcessError as e:
            print(f"     Failed to file issue: {e.stderr}")

    # Update tracking state
    new_state["issues_filed"] = issues_filed
    TRACKING_FILE.write_text(json.dumps(new_state, indent=2))
    print(f"\n==> Done. {len(issues_filed)} issues filed. State updated.")


if __name__ == "__main__":
    main()
