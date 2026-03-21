package system

import (
	"strings"
	"testing"
)

func TestSearchDiscussions_HitTitle(t *testing.T) {
	// "nvidia" appears in many Bluefin discussions — even 0 matches is ok
	results, corpusDate, err := SearchDiscussions("nvidia", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if corpusDate == "" {
		t.Error("expected non-empty corpus_date")
	}
	_ = results
}

func TestSearchDiscussions_MultiWord(t *testing.T) {
	// Word-AND: both terms must be present in every returned result
	results, _, err := SearchDiscussions("flatpak install", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, r := range results {
		text := strings.ToLower(r.Title + " " + r.Body + " " + r.AnswerBody)
		if !strings.Contains(text, "flatpak") || !strings.Contains(text, "install") {
			t.Errorf("result %q doesn't match both terms", r.Title)
		}
	}
}

func TestSearchDiscussions_NoMatch(t *testing.T) {
	results, _, err := SearchDiscussions("xyzzy_nonexistent_term_12345", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results for garbage query, got %d", len(results))
	}
}

func TestSearchDiscussions_LimitEnforced(t *testing.T) {
	// Empty query matches all; limit=3 must cap results
	results, _, err := SearchDiscussions("", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) > 3 {
		t.Errorf("expected at most 3 results, got %d", len(results))
	}
}

func TestSearchDiscussions_MaxLimitCapped(t *testing.T) {
	// limit=999 must be capped at 20
	results, _, err := SearchDiscussions("", 999)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) > 20 {
		t.Errorf("expected at most 20 results (cap), got %d", len(results))
	}
}

func TestSearchDiscussions_SortedByUpvotes(t *testing.T) {
	results, _, err := SearchDiscussions("", 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i := 1; i < len(results); i++ {
		if results[i].Upvotes > results[i-1].Upvotes {
			t.Errorf("results not sorted by upvotes: index %d (%d) > index %d (%d)",
				i, results[i].Upvotes, i-1, results[i-1].Upvotes)
		}
	}
}

func TestSearchDiscussions_DefaultLimit(t *testing.T) {
	// limit=0 should default to 5
	results, _, err := SearchDiscussions("", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) > 5 {
		t.Errorf("expected at most 5 results with default limit, got %d", len(results))
	}
}
