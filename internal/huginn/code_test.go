package huginn

import (
	"context"
	"os/exec"
	"strings"
	"testing"
)

func fence(lang, code string) string {
	return "Here you go:\n\n```" + lang + "\n" + code + "\n```\n\nDone."
}

func TestCheckCode(t *testing.T) {
	ctx := context.Background()
	ok := []string{
		fence("go", "package main\n\nfunc main() { println(\"hi\") }"),
		fence("go", "func add(a, b int) int { return a + b }"), // declarations without a package
		fence("go", "x := 1\nfmt.Println(x)"),                  // a function body
		fence("json", `{"name": "studio", "port": 7332}`),
		fence("go", "func f() {\n\t...\n}"), // a sketch is skipped
		fence("rust", "fn main( {"),         // a language that isn't checked
		fence("", "this is { not code"),     // an untagged block
	}
	for _, a := range ok {
		if issues := CheckCode(ctx, a); len(issues) != 0 {
			t.Errorf("CheckCode(%q) = %v", a, issues)
		}
	}
	bad := map[string]string{
		fence("go", "package main\n\nfunc main() {\n\tprintln(\"hi\"\n}"): "go block 1, line 4",
		fence("json", "{\n  \"a\": 1,\n}"):                                "json block 1, line 3",
	}
	for a, want := range bad {
		issues := CheckCode(ctx, a)
		if len(issues) != 1 || !strings.HasPrefix(issues[0].Text(), want) {
			t.Errorf("CheckCode(%q) = %v, want %s", a, issues, want)
		}
	}
	// Blocks are counted across the answer, checked or not.
	two := fence("rust", "fn main() {}") + fence("json", "[1, 2,]")
	if issues := CheckCode(ctx, two); len(issues) != 1 || issues[0].Block.Index != 2 {
		t.Fatalf("second block: %v", issues)
	}
}

func TestCheckCodeWithInstalledParsers(t *testing.T) {
	ctx := context.Background()
	if _, err := exec.LookPath("python3"); err == nil {
		if issues := CheckCode(ctx, fence("python", "def f(x):\n    return x +\n")); len(issues) != 1 || issues[0].Line != 2 {
			t.Errorf("python: %v", issues)
		}
		// The parser never runs the code.
		if issues := CheckCode(ctx, fence("python", "import os\nos.system('touch /tmp/should-not-exist-yggdrasil')")); len(issues) != 0 {
			t.Errorf("valid python: %v", issues)
		}
		if exec.Command("test", "-e", "/tmp/should-not-exist-yggdrasil").Run() == nil {
			t.Fatal("the checked code ran")
		}
	}
	if _, err := exec.LookPath("bash"); err == nil {
		if issues := CheckCode(ctx, fence("bash", "if true; then\n  echo hi\n")); len(issues) != 1 {
			t.Errorf("bash: %v", issues)
		}
		if issues := CheckCode(ctx, fence("sh", "for f in *.txt; do echo \"$f\"; done")); len(issues) != 0 {
			t.Errorf("valid shell: %v", issues)
		}
	}
}
