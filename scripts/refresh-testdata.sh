#!/usr/bin/env bash
# refresh-testdata.sh — Regenerate testdata/ and internal/seed/units.json from
# the actual Bluefin source repos on GitHub.
#
# Sources of truth:
#   projectbluefin/common  — ujust recipes (.just files), systemd units
#   ublue-os/bluefin       — image refs, variant definitions, systemd units
#   ublue-os/bluefin-lts   — LTS variant (currently no extra systemd units)
#
# Requirements: gh CLI (authenticated), curl, jq
# Usage: bash scripts/refresh-testdata.sh

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TESTDATA="$REPO_ROOT/testdata"
SEED="$REPO_ROOT/internal/seed"

# ── Helpers ─────────────────────────────────────────────────────────────────

die() { echo "ERROR: $*" >&2; exit 1; }

require_cmd() {
    command -v "$1" >/dev/null 2>&1 || die "'$1' is required but not found in PATH"
}

require_cmd gh
require_cmd curl
require_cmd jq

# Fetch a raw file from GitHub via the API, return its content on stdout.
# Usage: gh_raw <owner/repo> <path>
gh_raw() {
    local repo="$1" path="$2"
    local url
    url=$(gh api "repos/${repo}/contents/${path}" --jq '.download_url' 2>/dev/null) || {
        echo "" ; return 1
    }
    curl -sf "$url"
}

# List files in a GitHub directory, filtered by extension.
# Usage: gh_list_files <owner/repo> <dir> [ext]
# Outputs one JSON object per line: {"name":"...", "download_url":"..."}
gh_list_files() {
    local repo="$1" dir="$2" ext="${3:-}"
    gh api "repos/${repo}/contents/${dir}" 2>/dev/null \
        | jq -c --arg ext "$ext" \
            '.[] | select(.type=="file") | select($ext=="" or (.name | endswith($ext))) | {name:.name, url:.download_url}'
}

echo "==> Fetching SHAs from upstream repos..."

SHA_COMMON=$(gh api repos/projectbluefin/common/commits/main --jq '.sha')
SHA_BLUEFIN=$(gh api repos/ublue-os/bluefin/commits/main --jq '.sha')
SHA_LTS=$(gh api repos/ublue-os/bluefin-lts/commits/main --jq '.sha')

echo "    projectbluefin/common  @ ${SHA_COMMON}"
echo "    ublue-os/bluefin       @ ${SHA_BLUEFIN}"
echo "    ublue-os/bluefin-lts   @ ${SHA_LTS}"

# ── Section 1: ujust recipes → testdata/ujust-list.txt ──────────────────────

echo ""
echo "==> Fetching ujust recipes from projectbluefin/common..."

# Directories containing recipe files (excluding 00-entry.just which is the router)
JUST_DIRS=(
    "system_files/shared/usr/share/ublue-os/just"
    "system_files/bluefin/usr/share/ublue-os/just"
)

# Parse a .just file from stdin.  Outputs "recipe-name\tdescription" lines.
# Rules:
#   - Skip lines beginning with _ (private recipes)
#   - Skip lines beginning with [group(
#   - A recipe line matches: ^[a-zA-Z][a-zA-Z0-9_-]* *(\S.*)?:
#   - The description is taken from the last # comment line immediately
#     preceding the recipe (ignoring [group(...)] attribute lines)
parse_just_recipes() {
    local prev_comment="" line name description
    while IFS= read -r line; do
        # Track comment lines
        if [[ "$line" =~ ^[[:space:]]*#[[:space:]]*(.*) ]]; then
            prev_comment="${BASH_REMATCH[1]}"
            continue
        fi
        # Skip [group(...)] attribute lines (but preserve prev_comment)
        if [[ "$line" =~ ^\[group\( ]]; then
            continue
        fi
        # Match recipe definitions: name followed by optional params then colon
        # Exclude private recipes (_name) and aliases (alias keyword)
        if [[ "$line" =~ ^([a-zA-Z][a-zA-Z0-9_-]*)([^:]*): ]]; then
            name="${BASH_REMATCH[1]}"
            description="$prev_comment"
            printf '%s\t%s\n' "$name" "$description"
        fi
        # Any non-comment, non-attribute line resets the comment buffer
        prev_comment=""
    done
}

# Collect all recipes into an associative array (dedup by name, first wins)
declare -A RECIPE_DESC

for dir in "${JUST_DIRS[@]}"; do
    while IFS= read -r entry; do
        fname=$(echo "$entry" | jq -r '.name')
        furl=$(echo "$entry" | jq -r '.url')

        # Skip the entry-point file and private/numeric-prefixed names
        [[ "$fname" == "00-entry.just" ]] && continue

        echo "    Parsing $fname from $dir..."
        content=$(curl -sf "$furl") || { echo "    WARNING: could not fetch $furl" >&2; continue; }

        while IFS=$'\t' read -r rname rdesc; do
            # First definition wins (shared takes precedence over bluefin-specific)
            if [[ -z "${RECIPE_DESC[$rname]+x}" ]]; then
                RECIPE_DESC["$rname"]="$rdesc"
            fi
        done < <(echo "$content" | parse_just_recipes)

    done < <(gh_list_files "projectbluefin/common" "$dir" ".just")
done

# Write ujust-list.txt in the format that matches `ujust --list` output
UJUST_OUT="$TESTDATA/ujust-list.txt"
{
    echo "Available recipes:"
    # Sort recipe names for stable output
    for rname in $(echo "${!RECIPE_DESC[@]}" | tr ' ' '\n' | sort); do
        rdesc="${RECIPE_DESC[$rname]}"
        if [[ -n "$rdesc" ]]; then
            printf '    %-40s # %s\n' "$rname" "$rdesc"
        else
            printf '    %s\n' "$rname"
        fi
    done
} > "$UJUST_OUT"

RECIPE_COUNT=${#RECIPE_DESC[@]}
echo "    Wrote $RECIPE_COUNT recipes to testdata/ujust-list.txt"

# ── Section 2: systemd units → internal/seed/units.json ─────────────────────

echo ""
echo "==> Fetching systemd units..."

# Unit source definitions: repo, path, variant
# Format: "repo|systemd_dir|variant"
UNIT_SOURCES=(
    "projectbluefin/common|system_files/shared/usr/lib/systemd/system|all"
    "projectbluefin/common|system_files/shared/usr/lib/systemd/user|all"
    "projectbluefin/common|system_files/bluefin/usr/lib/systemd/system|all"
    "projectbluefin/common|system_files/bluefin/usr/lib/systemd/user|all"
    "ublue-os/bluefin|system_files/shared/usr/lib/systemd/system|all"
    "ublue-os/bluefin|system_files/dx/usr/lib/systemd/system|dx"
)

# Build units JSON using a temp file for jq accumulation
UNITS_JSON='{}'

for source in "${UNIT_SOURCES[@]}"; do
    IFS='|' read -r repo dir variant <<< "$source"

    # Use correct SHA for attribution comment
    case "$repo" in
        "projectbluefin/common") sha="$SHA_COMMON" ;;
        "ublue-os/bluefin")      sha="$SHA_BLUEFIN" ;;
        "ublue-os/bluefin-lts")  sha="$SHA_LTS" ;;
        *) sha="unknown" ;;
    esac

    echo "    Scanning $repo/$dir (variant=$variant)..."

    while IFS= read -r entry; do
        fname=$(echo "$entry" | jq -r '.name')
        furl=$(echo "$entry" | jq -r '.url')

        # Only process .service files
        [[ "$fname" == *.service ]] || continue

        content=$(curl -sf "$furl") || { echo "    WARNING: could not fetch $fname" >&2; continue; }

        # Extract Description= field (first occurrence)
        raw_desc=$(echo "$content" | grep -m1 '^Description=' | sed 's/^Description=//' | tr -d '\r') || raw_desc=""

        if [[ -z "$raw_desc" ]]; then
            raw_desc="(no description)"
        fi

        # Append source attribution
        description="${raw_desc} Source: ${repo}@${sha:0:7}"

        echo "      + $fname: $raw_desc"

        # Add to JSON (later entries override earlier ones for same unit name — last source wins
        # for shared units; variant-specific sources add their variant correctly)
        UNITS_JSON=$(echo "$UNITS_JSON" | jq \
            --arg name "$fname" \
            --arg variant "$variant" \
            --arg desc "$description" \
            '.[$name] = {"name": $name, "variant": $variant, "description": $desc}')

    done < <(gh_list_files "$repo" "$dir" ".service")
done

UNIT_COUNT=$(echo "$UNITS_JSON" | jq 'length')
echo "    Collected $UNIT_COUNT units"

# Write internal/seed/units.json
mkdir -p "$SEED"
{
    jq -n \
        --argjson units "$UNITS_JSON" \
        --arg sha "$SHA_COMMON" \
        --arg generated_at "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
        '{
            "version": "1",
            "source_sha": $sha,
            "units": $units
        }'
} > "$SEED/units.json"

echo "    Wrote $UNIT_COUNT units to internal/seed/units.json"

# ── Section 3: declarative flatpaks → testdata/flatpak-list.txt ─────────────

echo ""
echo "==> Fetching declarative flatpak list from projectbluefin/common..."

# The canonical flatpak list lives in system-flatpaks.Brewfile
FLATPAK_BREWFILE="system_files/bluefin/usr/share/ublue-os/homebrew/system-flatpaks.Brewfile"

flatpak_content=$(gh_raw "projectbluefin/common" "$FLATPAK_BREWFILE") || \
    die "Could not fetch system-flatpaks.Brewfile"

FLATPAK_OUT="$TESTDATA/flatpak-list.txt"
{
    # Header comment matching the original file format expectations in tests
    # The actual `flatpak list --columns=application,version` output has no header
    # We use a version placeholder since source doesn't include runtime versions
    while IFS= read -r line; do
        # Match lines like: flatpak "com.example.App"
        if [[ "$line" =~ ^flatpak[[:space:]]+\"([^\"]+)\" ]]; then
            app_id="${BASH_REMATCH[1]}"
            printf '%s\t1.0.0\n' "$app_id"
        fi
    done <<< "$flatpak_content"
} > "$FLATPAK_OUT"

FLATPAK_COUNT=$(wc -l < "$FLATPAK_OUT")
echo "    Wrote $FLATPAK_COUNT flatpaks to testdata/flatpak-list.txt"

# ── Section 4: bootc-status golden files → testdata/bootc-status*.json ──────

echo ""
echo "==> Writing bootc-status golden files..."

# Helper to generate a realistic sha256 digest deterministically from a seed string
# (We can't easily fetch real GHCR digests without container auth; use stable
#  placeholder digests derived from the upstream SHAs so they change when upstream does.)
make_digest() {
    local seed="$1"
    # Use the bluefin SHA + seed to produce a stable 64-hex digest
    echo -n "${SHA_BLUEFIN}${seed}" | sha256sum | awk '{print $1}'
}

write_bootc_status() {
    local outfile="$1"
    local image_ref="$2"
    local digest_seed="$3"
    local booted_digest rollback_digest

    booted_digest=$(make_digest "${digest_seed}-booted")
    rollback_digest=$(make_digest "${digest_seed}-rollback")

    jq -n \
        --arg image "$image_ref" \
        --arg booted "sha256:${booted_digest}" \
        --arg rollback "sha256:${rollback_digest}" \
        '{
            "status": {
                "booted": {
                    "image": {
                        "image": {
                            "image": $image,
                            "transport": "registry"
                        },
                        "imageDigest": $booted
                    },
                    "incompatible": false,
                    "pinned": false
                },
                "staged": null,
                "rollback": {
                    "image": {
                        "image": {
                            "image": $image,
                            "transport": "registry"
                        },
                        "imageDigest": $rollback
                    }
                }
            }
        }' > "$outfile"

    echo "    Wrote $outfile"
}

write_bootc_status "$TESTDATA/bootc-status.json"        "ghcr.io/ublue-os/bluefin:stable"        "bluefin-stable"
write_bootc_status "$TESTDATA/bootc-status-dx.json"     "ghcr.io/ublue-os/bluefin-dx:stable"     "bluefin-dx-stable"
write_bootc_status "$TESTDATA/bootc-status-nvidia.json" "ghcr.io/ublue-os/bluefin-nvidia:stable" "bluefin-nvidia-stable"

# ── Section 5: refresh-manifest.json → testdata/refresh-manifest.json ───────

echo ""
echo "==> Writing refresh manifest..."

GENERATED_AT=$(date -u +%Y-%m-%dT%H:%M:%SZ)

jq -n \
    --arg generated_at "$GENERATED_AT" \
    --arg sha_common "$SHA_COMMON" \
    --arg sha_bluefin "$SHA_BLUEFIN" \
    --arg sha_lts "$SHA_LTS" \
    '{
        "generated_at": $generated_at,
        "sources": {
            "projectbluefin/common": $sha_common,
            "ublue-os/bluefin": $sha_bluefin,
            "ublue-os/bluefin-lts": $sha_lts
        }
    }' > "$TESTDATA/refresh-manifest.json"

echo "    Wrote testdata/refresh-manifest.json"

# ── Section 6: GitHub Discussions → internal/seed/discussions.json ──────────

echo ""
echo "==> Fetching ublue-os/bluefin Discussions..."
CORPUS_DATE=$(date -u +%Y-%m-%d)
python3 "$REPO_ROOT/scripts/fetch-discussions.py" --output "$SEED/discussions.json" --corpus-date "$CORPUS_DATE"
DISCUSSION_COUNT=$(python3 -c "import json; d=json.load(open('$SEED/discussions.json')); print(len(d['discussions']))")
echo "    Wrote $DISCUSSION_COUNT discussions to internal/seed/discussions.json"

# ── Done ─────────────────────────────────────────────────────────────────────

echo ""
echo "==> Done."
echo "    Recipes:     $RECIPE_COUNT  (testdata/ujust-list.txt)"
echo "    Units:       $UNIT_COUNT    (internal/seed/units.json)"
echo "    Flatpaks:    $FLATPAK_COUNT (testdata/flatpak-list.txt)"
echo "    Discussions: $DISCUSSION_COUNT (internal/seed/discussions.json)"
echo ""
echo "Run 'go test -race ./...' to verify."
