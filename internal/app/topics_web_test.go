package app

import (
	"errors"
	"testing"

	"github.com/yeixio/toskar-core/internal/tools/internet"
	"github.com/yeixio/toskar-core/pkg/contracts"
)

// A profile's web limit keeps searches and pages on its sites, with its
// words added to each search (#345).
func TestTopicsWebLimit(t *testing.T) {
	limit := &contracts.TopicPolicy{StaysOn: "Tires", WebSites: []string{"danastires.com", "michelin.com"}, WebKeywords: []string{"tires"}}

	args, err := limitWebArgs(limit, "web.search", map[string]any{"query": "winter options", "count": 5})
	if err != nil || args["query"] != "winter options tires site:danastires.com OR site:michelin.com" || args["count"] != 5 {
		t.Fatalf("search: %v %v", args, err)
	}
	if args, _ := limitWebArgs(limit, "internet.search", map[string]any{"query": "Winter TIRES"}); args["query"] != "Winter TIRES site:danastires.com OR site:michelin.com" {
		t.Fatalf("a word already there: %v", args)
	}
	for u, ok := range map[string]bool{
		"https://danastires.com/hours":      true,
		"https://www.michelin.com/x":        true,
		"https://shop.danastires.com/":      true,
		"https://danastires.com.evil.io/":   false,
		"https://notdanastires.com/":        false,
		"file:///etc/passwd":                false,
		"https://user@danastires.com@evil/": false,
	} {
		_, err := limitWebArgs(limit, "browser.open", map[string]any{"url": u})
		if (err == nil) != ok {
			t.Errorf("%s: %v", u, err)
		}
	}

	res, _ := limitWebResult(limit, "internet.search", map[string]any{"results": []internet.Result{
		{URL: "https://danastires.com/winter"}, {URL: "https://costco.com/tires"},
	}})
	if list := res["results"].([]internet.Result); len(list) != 1 || list[0].URL != "https://danastires.com/winter" {
		t.Fatalf("results: %+v", list)
	}
	res, _ = limitWebResult(limit, "internet.search", map[string]any{"results": []any{
		map[string]any{"url": "https://costco.com/tires"}, map[string]any{"url": "https://michelin.com/a"},
	}})
	if list := res["results"].([]any); len(list) != 1 {
		t.Fatalf("cached results: %+v", list)
	}
	if _, err := limitWebResult(limit, "browser.click", map[string]any{"url": "https://costco.com/"}); !errors.Is(err, errOffSite) {
		t.Fatalf("a click off the sites: %v", err)
	}

	// No sites: words only, and any page.
	words := &contracts.TopicPolicy{StaysOn: "Tires", WebKeywords: []string{"tires"}}
	if _, err := limitWebArgs(words, "internet.open", map[string]any{"url": "https://costco.com"}); err != nil {
		t.Fatal(err)
	}
	if args, _ := limitWebArgs(nil, "internet.search", map[string]any{"query": "x"}); args["query"] != "x" {
		t.Fatal("no topics")
	}
}
