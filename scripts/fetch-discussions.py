#!/usr/bin/env python3
"""Fetch ublue-os/bluefin GitHub Discussions and write as seed JSON.

Fetches two pages (recent + oldest) of discussions, deduplicates by URL,
filters to answered OR upvotes >= 2, caps at 200, and writes the result to
the specified output file.
"""

import argparse
import json
import subprocess
import sys
from datetime import datetime, timezone

QUERY_RECENT = """
{
  repository(owner: "ublue-os", name: "bluefin") {
    discussions(first: 100, orderBy: {field: UPDATED_AT, direction: DESC}) {
      nodes {
        title
        url
        upvoteCount
        createdAt
        body
        category { name }
        answer { body }
      }
    }
  }
}
"""

QUERY_OLDEST = """
{
  repository(owner: "ublue-os", name: "bluefin") {
    discussions(first: 100, orderBy: {field: UPDATED_AT, direction: ASC}) {
      nodes {
        title
        url
        upvoteCount
        createdAt
        body
        category { name }
        answer { body }
      }
    }
  }
}
"""


def gh_graphql(query: str) -> dict:
    result = subprocess.run(
        ["gh", "api", "graphql", "-f", f"query={query}"],
        capture_output=True,
        text=True,
        check=True,
    )
    return json.loads(result.stdout)


def truncate(s: str, max_len: int = 2000) -> str:
    if not s:
        return ""
    return s[:max_len] + ("..." if len(s) > max_len else "")


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", required=True, help="Output JSON file path")
    parser.add_argument(
        "--corpus-date",
        default=datetime.now(timezone.utc).strftime("%Y-%m-%d"),
        help="Date string for corpus_date field (default: today UTC)",
    )
    args = parser.parse_args()

    print("  Fetching page 1 (most recently updated)...")
    data1 = gh_graphql(QUERY_RECENT)
    nodes1 = data1["data"]["repository"]["discussions"]["nodes"]

    print("  Fetching page 2 (oldest)...")
    data2 = gh_graphql(QUERY_OLDEST)
    nodes2 = data2["data"]["repository"]["discussions"]["nodes"]

    # Merge, deduplicate by URL
    seen: set[str] = set()
    all_nodes = []
    for n in nodes1 + nodes2:
        if n["url"] not in seen:
            seen.add(n["url"])
            all_nodes.append(n)

    # Filter: keep answered OR upvotes >= 2
    filtered = [
        n for n in all_nodes if n["answer"] is not None or n["upvoteCount"] >= 2
    ]
    print(
        f"  Total unique: {len(all_nodes)}, after filter (answered or upvotes>=2): {len(filtered)}"
    )

    # Sort by upvotes desc for consistent ordering
    filtered.sort(key=lambda x: -x["upvoteCount"])

    # Cap at 200
    filtered = filtered[:200]

    discussions = []
    for n in filtered:
        discussions.append(
            {
                "title": n["title"],
                "url": n["url"],
                "category": n["category"]["name"],
                "upvotes": n["upvoteCount"],
                "answered": n["answer"] is not None,
                "answer_body": truncate(n["answer"]["body"] if n["answer"] else ""),
                "body": truncate(n["body"]),
                "created_at": n["createdAt"],
            }
        )

    output = {
        "version": "1",
        "fetched_at": datetime.now(timezone.utc).isoformat(),
        "source": "ublue-os/bluefin",
        "corpus_date": args.corpus_date,
        "discussions": discussions,
    }

    with open(args.output, "w") as f:
        json.dump(output, f, indent=2)

    print(f"  Wrote {len(discussions)} discussions to {args.output}")


if __name__ == "__main__":
    main()
