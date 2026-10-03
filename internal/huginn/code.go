package huginn

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go/parser"
	"go/scanner"
	"go/token"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// CodeBlock is a fenced block of code in an answer.
type CodeBlock struct {
	// Index counts the answer's fenced blocks from 1.
	Index int
	// Lang is the language the fence names, normalized: go, python, json,
	// or shell; other languages are not checked.
	Lang string
	Code string
}

// CodeIssue is a syntax error in a code block.
type CodeIssue struct {
	Block CodeBlock
	// Line is the line in the block, from 1, or 0 when unknown.
	Line    int
	Message string
}

// Text is the issue as the model and the person read it, such as
// "python block 2, line 3: invalid syntax".
func (i CodeIssue) Text() string {
	if i.Line > 0 {
		return fmt.Sprintf("%s block %d, line %d: %s", i.Block.Lang, i.Block.Index, i.Line, i.Message)
	}
	return fmt.Sprintf("%s block %d: %s", i.Block.Lang, i.Block.Index, i.Message)
}

var fenceRe = regexp.MustCompile("(?ms)^[ \t]*```[ \t]*([A-Za-z0-9_+#.-]*)[^\n]*\n(.*?)^[ \t]*```[ \t]*$")

var (
	packageRe  = regexp.MustCompile(`(?m)^\s*package\s+\w+`)
	bashLineRe = regexp.MustCompile(`line (\d+): (.*)`)
)

// checkedLangs maps fence names to the languages CheckCode can parse.
var checkedLangs = map[string]string{
	"go": "go", "golang": "go",
	"python": "python", "py": "python", "python3": "python",
	"json": "json",
	"sh":   "shell", "bash": "shell", "shell": "shell", "zsh": "shell",
}

// CodeBlocks are the answer's fenced code blocks in languages CheckCode
// can parse. Blocks with placeholders such as "..." are left out, since a
// sketch isn't meant to compile.
func CodeBlocks(answer string) []CodeBlock {
	var out []CodeBlock
	for i, m := range fenceRe.FindAllStringSubmatch(answer, -1) {
		lang, ok := checkedLangs[strings.ToLower(m[1])]
		if !ok || strings.TrimSpace(m[2]) == "" {
			continue
		}
		if (lang != "python" && strings.Contains(m[2], "...")) || strings.Contains(m[2], "…") {
			continue
		}
		out = append(out, CodeBlock{Index: i + 1, Lang: lang, Code: m[2]})
	}
	return out
}

// CheckCode parses the answer's code blocks and returns their syntax
// errors. It never runs the code: Go and JSON are parsed here, Python by
// its own parser (ast.parse) and shell by bash -n, each when installed.
func CheckCode(ctx context.Context, answer string) []CodeIssue {
	var issues []CodeIssue
	for _, b := range CodeBlocks(answer) {
		var line int
		var msg string
		switch b.Lang {
		case "go":
			line, msg = checkGo(b.Code)
		case "json":
			line, msg = checkJSON(b.Code)
		case "python":
			line, msg = checkPython(ctx, b.Code)
		case "shell":
			line, msg = checkShell(ctx, b.Code)
		}
		if msg != "" {
			issues = append(issues, CodeIssue{Block: b, Line: line, Message: msg})
		}
	}
	return issues
}

// checkGo parses a file, or, without a package clause, the snippet as a
// file's declarations and then as a function body.
func checkGo(code string) (int, string) {
	if packageRe.MatchString(code) {
		return goErr(code, 0)
	}
	if _, msg := goErr("package p\n"+code, 1); msg == "" {
		return 0, ""
	}
	return goErr("package p\nfunc _() {\n"+code+"\n}", 2)
}

func goErr(src string, offset int) (int, string) {
	_, err := parser.ParseFile(token.NewFileSet(), "answer.go", src, parser.AllErrors)
	if err == nil {
		return 0, ""
	}
	if list, ok := err.(scanner.ErrorList); ok && len(list) > 0 {
		return max(list[0].Pos.Line-offset, 0), list[0].Msg
	}
	return 0, err.Error()
}

func checkJSON(code string) (int, string) {
	var v any
	err := json.Unmarshal([]byte(code), &v)
	if err == nil {
		return 0, ""
	}
	if se, ok := err.(*json.SyntaxError); ok {
		return strings.Count(code[:min(int(se.Offset), len(code))], "\n") + 1, se.Error()
	}
	return 0, err.Error()
}

// pythonCheck parses stdin with Python's own parser, without running it.
const pythonCheck = `import ast, sys
try:
    ast.parse(sys.stdin.read())
except SyntaxError as e:
    print(e.lineno or 0, e.msg)
    sys.exit(1)
`

func checkPython(ctx context.Context, code string) (int, string) {
	py, err := exec.LookPath("python3")
	if err != nil {
		return 0, ""
	}
	return runCheck(ctx, code, py, "-I", "-S", "-c", pythonCheck)
}

// checkShell reads the script with bash -n, which parses without running.
func checkShell(ctx context.Context, code string) (int, string) {
	sh, err := exec.LookPath("bash")
	if err != nil {
		return 0, ""
	}
	line, msg := runCheck(ctx, code, sh, "-n")
	if msg != "" {
		// bash: line 3: syntax error near unexpected token `fi'
		if m := bashLineRe.FindStringSubmatch(msg); m != nil {
			n, _ := strconv.Atoi(m[1])
			return n, m[2]
		}
	}
	return line, msg
}

// runCheck runs a parser on code and reads "line message" or a message
// from its output when it fails. A parser that can't run says nothing.
func runCheck(ctx context.Context, code, name string, args ...string) (int, string) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = strings.NewReader(code)
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err := cmd.Run()
	if err == nil || ctx.Err() != nil {
		return 0, ""
	}
	if _, ok := err.(*exec.ExitError); !ok {
		return 0, ""
	}
	text := strings.TrimSpace(out.String())
	if first, rest, ok := strings.Cut(text, " "); ok {
		if n, err := strconv.Atoi(first); err == nil {
			return n, rest
		}
	}
	if i := strings.LastIndex(text, "\n"); i >= 0 {
		text = text[i+1:]
	}
	return 0, text
}

// DescribeCode lists code issues for the model, one per line.
func DescribeCode(issues []CodeIssue) string {
	var b strings.Builder
	for _, i := range issues {
		b.WriteString("- " + i.Text() + "\n")
	}
	return b.String()
}
