package search

import (
	"context"
	"testing"
	"time"
)

func TestWebSearch(t *testing.T) {
	engine := NewEngine()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	results, err := engine.Search(ctx, "meta ai developer hackathon 2026", 3)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) == 0 {
		t.Fatalf("Expected at least 1 search result, got 0")
	}

	t.Logf("Found %d search results:", len(results))
	for i, r := range results {
		t.Logf("[%d] %s (%s)", i+1, r.Title, r.URL)
	}

	formatted := FormatForLLM(results)
	if len(formatted) == 0 {
		t.Fatalf("Formatted output was empty")
	}
}

