package api

import (
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"gopkg.in/yaml.v3"
)

type openAPIDoc struct {
	Paths      map[string]map[string]openAPIOperation `yaml:"paths"`
	Components struct {
		Schemas map[string]any `yaml:"schemas"`
	} `yaml:"components"`
}

type openAPIOperation struct {
	OperationID string `yaml:"operationId"`
	Parameters  []struct {
		Name string `yaml:"name"`
		In   string `yaml:"in"`
	} `yaml:"parameters"`
}

var openAPIMethods = map[string]bool{"get": true, "post": true, "put": true, "patch": true, "delete": true}

func loadOpenAPI(t *testing.T) (openAPIDoc, string) {
	t.Helper()
	raw, err := os.ReadFile("../../api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var doc openAPIDoc
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("api/openapi.yaml: %v", err)
	}
	return doc, string(raw)
}

// api/openapi.yaml describes every route the daemon serves, and nothing it
// does not, so the spec cannot fall behind the router again.
func TestOpenAPICoversEveryRoute(t *testing.T) {
	doc, _ := loadOpenAPI(t)
	srv := NewServer(Dependencies{})

	served := map[string]bool{}
	err := srv.router.Walk(func(route *mux.Route, _ *mux.Router, _ []*mux.Route) error {
		path, err := route.GetPathTemplate()
		if err != nil {
			return nil
		}
		methods, err := route.GetMethods()
		if err != nil {
			return nil // a prefix with no methods of its own
		}
		for _, m := range methods {
			if m != http.MethodOptions {
				served[strings.ToLower(m)+" "+path] = true
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	described := map[string]bool{}
	for path, ops := range doc.Paths {
		for m := range ops {
			if openAPIMethods[m] {
				described[m+" "+path] = true
			}
		}
	}

	var missing, extra []string
	for k := range served {
		if !described[k] {
			missing = append(missing, k)
		}
	}
	for k := range described {
		if !served[k] {
			extra = append(extra, k)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	if len(missing) > 0 {
		t.Errorf("routes missing from api/openapi.yaml:\n  %s", strings.Join(missing, "\n  "))
	}
	if len(extra) > 0 {
		t.Errorf("api/openapi.yaml describes routes the daemon does not serve:\n  %s", strings.Join(extra, "\n  "))
	}
}

// Every operation has a unique id, declares its path parameters, and refers
// only to schemas that exist.
func TestOpenAPIIsConsistent(t *testing.T) {
	doc, raw := loadOpenAPI(t)
	ids := map[string]string{}
	param := regexp.MustCompile(`\{([^}]+)\}`)
	for path, ops := range doc.Paths {
		for m, op := range ops {
			if !openAPIMethods[m] {
				continue
			}
			where := m + " " + path
			if op.OperationID == "" {
				t.Errorf("%s has no operationId", where)
			} else if prev, dup := ids[op.OperationID]; dup {
				t.Errorf("operationId %q is used by %s and %s", op.OperationID, prev, where)
			}
			ids[op.OperationID] = where
			declared := map[string]bool{}
			for _, p := range op.Parameters {
				if p.In == "path" {
					declared[p.Name] = true
				}
			}
			for _, match := range param.FindAllStringSubmatch(path, -1) {
				if !declared[match[1]] {
					t.Errorf("%s does not declare path parameter %q", where, match[1])
				}
			}
		}
	}
	for _, match := range regexp.MustCompile(`#/components/schemas/([A-Za-z0-9_]+)`).FindAllStringSubmatch(raw, -1) {
		if _, ok := doc.Components.Schemas[match[1]]; !ok {
			t.Errorf("schema %q is referenced but not defined", match[1])
		}
	}
}
