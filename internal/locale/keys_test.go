package locale

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var (
	// "notifications:notices.modelReady" and the like, anywhere in core.
	keyLiteral = regexp.MustCompile(`"([a-zA-Z]+:[a-zA-Z0-9_.]+)"`)
	// notice("modelReady", params, "modelReadyBody", params) and
	// computerNotice("computerOffline", name, "computerOfflineBody") in
	// internal/app: the first argument, and a later one after nil or a map.
	noticeTitle  = regexp.MustCompile(`\b(?:notice|computerNotice)\("([a-zA-Z]+)"`)
	noticeBody   = regexp.MustCompile(`(?:nil|\}|\)),\s*"([a-zA-Z]+Body)"`)
	computerCall = regexp.MustCompile(`\bcomputerNotice\("([a-zA-Z]+)"`)
)

// Every catalog key core writes text with has English text, so nobody sees
// a raw key in a notification.
func TestKeysCoreUsesAreInEnglish(t *testing.T) {
	english := load().text[Source]
	has := func(key string) bool {
		_, ok := english[key]
		_, one := english[key+"_one"]
		_, other := english[key+"_other"]
		return ok || one || other
	}
	used := map[string]string{}
	root := filepath.Join("..", "..")
	for _, dir := range []string{"internal", "pkg", "cmd"} {
		_ = filepath.WalkDir(filepath.Join(root, dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			src := string(raw)
			for _, m := range keyLiteral.FindAllStringSubmatch(src, -1) {
				// A key that ends in "." is completed at run time, such as
				// "notifications:categories." + category.
				if ns, _, _ := strings.Cut(m[1], ":"); hasNamespace(english, ns) && !strings.HasSuffix(m[1], ".") {
					used[m[1]] = path
				}
			}
			for _, re := range []*regexp.Regexp{noticeTitle, noticeBody} {
				for _, m := range re.FindAllStringSubmatch(src, -1) {
					used["notifications:notices."+m[1]] = path
				}
			}
			for _, m := range computerCall.FindAllStringSubmatch(src, -1) {
				used["notifications:notices."+m[1]+"Unnamed"] = path
			}
			return nil
		})
	}
	if len(used) < 20 {
		t.Fatalf("found only %d keys; is the scan reading the source?", len(used))
	}
	for key, where := range used {
		if !has(key) {
			t.Errorf("%s uses %s, which i18n/locales/en has no text for", where, key)
		}
	}
}

func hasNamespace(table map[string]string, ns string) bool {
	for k := range table {
		if strings.HasPrefix(k, ns+":") {
			return true
		}
	}
	return false
}
