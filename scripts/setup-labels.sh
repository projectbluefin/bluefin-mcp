#!/usr/bin/env bash
set -euo pipefail
REPO="projectbluefin/bluefin-mcp"

create_label() {
  local name="$1" color="$2" desc="$3"
  gh label create "$name" --color "$color" --description "$desc" --repo "$REPO" --force
}

# Classification
create_label "feature-idea"        "0075ca" "Proposed new capability for bluefin-mcp"
create_label "auto-generated"      "e4e669" "Created by the weekly Bluefin change tracker GHA"
create_label "needs-human-review"  "d93f0b" "Awaiting human approval before implementation"
create_label "approved"            "0e8a16" "Human approved — ready for agent implementation"
create_label "tdd-ready"           "1d76db" "Issue has enough detail for full TDD implementation"
create_label "has-tests"           "0e8a16" "Implementation includes full test coverage"
create_label "good-first-issue"    "7057ff" "Well-scoped, good starting point"

# Tier labels (which part of bluefin-mcp)
create_label "tier:atomic-os"      "c5def5" "Relates to bootc/OCI/atomic OS state tools"
create_label "tier:ujust"          "c5def5" "Relates to ujust recipe tooling"
create_label "tier:packages"       "c5def5" "Relates to flatpak/brew/distrobox tools"
create_label "tier:knowledge-store" "c5def5" "Relates to unit documentation store"
create_label "tier:new-tool"       "c5def5" "Proposes an entirely new MCP tool"

# Source labels (where in Bluefin the change originated)
create_label "source:common"       "f9d0c4" "Change from projectbluefin/common"
create_label "source:bluefin"      "f9d0c4" "Change from ublue-os/bluefin"
create_label "source:bluefin-lts"  "f9d0c4" "Change from ublue-os/bluefin-lts"

# Variant labels
create_label "variant:dx"          "bfd4f2" "Specific to Bluefin DX (developer) variant"
create_label "variant:nvidia"      "bfd4f2" "Specific to Bluefin Nvidia variant"
create_label "variant:aurora"      "bfd4f2" "Specific to Aurora variant"
create_label "variant:lts"         "bfd4f2" "Specific to Bluefin LTS variant"
create_label "variant:all"         "bfd4f2" "Applies to all Bluefin variants"

# Priority labels
create_label "priority:high"       "b60205" "High impact, implement soon"
create_label "priority:medium"     "fbca04" "Medium impact"
create_label "priority:low"        "e4e669" "Nice to have, low urgency"

echo "All labels created in $REPO"
