package app

import (
	"errors"
	"net/url"
	"strings"

	"github.com/yeixio/toskar-core/internal/profiles"
	"github.com/yeixio/toskar-core/internal/tools"
	"github.com/yeixio/toskar-core/internal/tools/internet"
	"github.com/yeixio/toskar-core/pkg/contracts"
)

// A profile's topic controls can limit the web (#345): searches get its
// words and reach only its sites, and pages open only on them. The limit
// applies where tools run, so a model's own call, Toskar's look-up, and a
// page followed from a result all keep to it.

// errOffSite is a page outside the profile's sites. The model reads it.
var errOffSite = errors.New("this assistant only opens pages on its own sites")

// onSites reports whether a page's address is on one of the sites or
// their subdomains.
func onSites(rawURL string, sites []string) bool {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return false
	}
	host := profiles.Site(u.Hostname())
	for _, s := range sites {
		if host == s || strings.HasSuffix(host, "."+s) {
			return true
		}
	}
	return false
}

// limitWebArgs applies the profile's web limit to a tool's arguments
// before it runs: a search gets the profile's words and sites, and a page
// off its sites isn't opened.
func limitWebArgs(t *contracts.TopicPolicy, toolID string, args map[string]any) (map[string]any, error) {
	if t == nil || (len(t.WebSites) == 0 && len(t.WebKeywords) == 0) {
		return args, nil
	}
	switch tools.Canonical(toolID) {
	case "internet.search":
		query, _ := args["query"].(string)
		query = strings.TrimSpace(query)
		lower := strings.ToLower(query)
		for _, w := range t.WebKeywords {
			if !strings.Contains(lower, strings.ToLower(w)) {
				query += " " + w
			}
		}
		if len(t.WebSites) > 0 {
			ops := make([]string, len(t.WebSites))
			for i, s := range t.WebSites {
				ops[i] = "site:" + s
			}
			query += " " + strings.Join(ops, " OR ")
		}
		out := make(map[string]any, len(args))
		for k, v := range args {
			out[k] = v
		}
		out["query"] = strings.TrimSpace(query)
		return out, nil
	case "internet.open", "browser.open":
		if len(t.WebSites) > 0 {
			if u, _ := args["url"].(string); !onSites(u, t.WebSites) {
				return nil, errOffSite
			}
		}
	}
	return args, nil
}

// limitWebResult keeps what a tool returned to the profile's sites: search
// results off them are dropped, and a browser that ended up off them gives
// nothing back.
func limitWebResult(t *contracts.TopicPolicy, toolID string, result map[string]any) (map[string]any, error) {
	if t == nil || len(t.WebSites) == 0 || result == nil {
		return result, nil
	}
	id := tools.Canonical(toolID)
	switch {
	case id == "internet.search":
		out := make(map[string]any, len(result))
		for k, v := range result {
			out[k] = v
		}
		switch list := result["results"].(type) {
		case []internet.Result:
			kept := []internet.Result{}
			for _, r := range list {
				if onSites(r.URL, t.WebSites) {
					kept = append(kept, r)
				}
			}
			out["results"] = kept
		case []any:
			kept := []any{}
			for _, r := range list {
				if m, ok := r.(map[string]any); ok {
					if u, _ := m["url"].(string); onSites(u, t.WebSites) {
						kept = append(kept, r)
					}
				}
			}
			out["results"] = kept
		}
		return out, nil
	case id == "internet.open" || strings.HasPrefix(id, "browser."):
		// A redirect or a click can lead elsewhere.
		if u, _ := result["url"].(string); u != "" && !onSites(u, t.WebSites) {
			return nil, errOffSite
		}
	}
	return result, nil
}
