package system

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/projectbluefin/bluefin-mcp/internal/seed"
)

// DiscussionEntry is a single GitHub Discussion from ublue-os/bluefin.
type DiscussionEntry struct {
	Title      string `json:"title"`
	URL        string `json:"url"`
	Category   string `json:"category"`
	Upvotes    int    `json:"upvotes"`
	Answered   bool   `json:"answered"`
	AnswerBody string `json:"answer_body"`
	Body       string `json:"body"`
	CreatedAt  string `json:"created_at"`
}

type discussionSeed struct {
	Version     string            `json:"version"`
	FetchedAt   string            `json:"fetched_at"`
	Source      string            `json:"source"`
	CorpusDate  string            `json:"corpus_date"`
	Discussions []DiscussionEntry `json:"discussions"`
}

// SearchDiscussions performs a case-insensitive word-AND search across the embedded
// discussion corpus. Each word in the query must appear somewhere in the entry
// (title, body, or answer_body). Results are sorted by upvotes descending.
// limit is capped at 20; if <= 0 it defaults to 5.
func SearchDiscussions(query string, limit int) ([]DiscussionEntry, string, error) {
	if limit <= 0 {
		limit = 5
	}
	if limit > 20 {
		limit = 20
	}

	var ds discussionSeed
	if err := json.Unmarshal(seed.Discussions, &ds); err != nil {
		return nil, "", fmt.Errorf("corrupt discussions seed: %w", err)
	}

	terms := strings.Fields(strings.ToLower(query))
	var results []DiscussionEntry
	for _, d := range ds.Discussions {
		if matchesAllTerms(terms, d) {
			results = append(results, d)
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Upvotes > results[j].Upvotes
	})
	if len(results) > limit {
		results = results[:limit]
	}
	return results, ds.CorpusDate, nil
}

func matchesAllTerms(terms []string, d DiscussionEntry) bool {
	if len(terms) == 0 {
		return true
	}
	text := strings.ToLower(d.Title + " " + d.Body + " " + d.AnswerBody)
	for _, term := range terms {
		if !strings.Contains(text, term) {
			return false
		}
	}
	return true
}
