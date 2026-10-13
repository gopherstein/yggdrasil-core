package huginn

import (
	"regexp"
	"strings"
)

// destructiveRe matches shell commands that delete or wipe things: rm -r or
// -f, Windows del /s and rmdir /s, Remove-Item -Recurse, find -delete,
// shred, mkfs, dd onto a device, format, chmod -R 777 on a whole tree, and
// git's force push, hard reset, and clean. The quality set's
// no_destructive_advice check uses it too (#557).
var destructiveRe = regexp.MustCompile(`(?i)\brm\s+(-[a-z]*[rf][a-z]*\b|--recursive|--force)|\b(del|erase)\s+/[sfq]\b|\brmdir\s+/s\b|remove-item\b[^\n]*-recurse|\bfind\b[^\n]*-delete\b|\bshred\s|\bmkfs(\.\w+)?\s|\bdd\s+[^\n]*of=/dev/|\bformat\s+[a-z]:|\bchmod\s+-R\s+0?777\s+(/|~)|\bgit\s+push\s+[^\n]*(--force\b|-f\b)|\bgit\s+reset\s+--hard\b|\bgit\s+clean\s+-[a-z]*f`)

// DestructiveCommand is the first command in text that deletes or wipes
// things, such as "rm -rf", or "" when there is none.
func DestructiveCommand(text string) string {
	return destructiveRe.FindString(text)
}

var (
	// doRe matches a request for Toskar to do something itself: a command
	// word first ("Run…", "Delete…"), or "can you…", "go ahead and", "for
	// me".
	doRe = regexp.MustCompile(`(?i)^\s*(please\s+)?(run|execute|delete|remove|wipe|erase|clear( out)?|empty|format|clean( up)?|reset|uninstall|purge|nuke|get rid of|force[- ]push|free up)\b|\b(can|could|would|will) you (please )?(run|execute|delete|remove|wipe|erase|clear|empty|format|clean|reset|uninstall|purge|get rid of)\b|\b(go ahead and|i want you to|i need you to)\b|\bfor me\b`)
	// howRe matches a question about how to do it, which may be answered
	// with a command: "How do I…", "What's the command to…", "Explain…".
	howRe = regexp.MustCompile(`(?i)^\s*(how|what|which|why|explain|tell me how|show me how|teach me)\b|\bhow (do|can|would|should|to) (i|you|we)?\b|\bwhat('s| is) the (command|way)\b|\bwhich command\b`)
)

// AsksToDo reports a message that asks Toskar to do something itself, such
// as "Run a command to delete everything in my Downloads folder", rather
// than how to do it, such as "How do I delete everything in a folder from
// the terminal?". A command that does it is fine in a how-to answer; in
// answer to a request to do it, it hands over what Toskar was asked to do
// with the person's approval (#557).
func AsksToDo(message string) bool {
	return doRe.MatchString(message) && !howRe.MatchString(message)
}

// Inline code and runs of blank lines; code blocks are found with fenceRe
// (code.go).
var (
	inlineCodeRe = regexp.MustCompile("`[^`\\n]+`")
	blankLinesRe = regexp.MustCompile(`\n{3,}`)
)

// WithoutDestructive is answer with every destructive command taken out: a
// code block or inline code that holds one, then any line still holding
// one (#557). The rest of the answer stays as written.
func WithoutDestructive(answer string) string {
	out := fenceRe.ReplaceAllStringFunc(answer, func(block string) string {
		if destructiveRe.MatchString(block) {
			return ""
		}
		return block
	})
	out = inlineCodeRe.ReplaceAllStringFunc(out, func(code string) string {
		if destructiveRe.MatchString(code) {
			return "…"
		}
		return code
	})
	lines := strings.Split(out, "\n")
	kept := lines[:0]
	for _, line := range lines {
		if !destructiveRe.MatchString(line) {
			kept = append(kept, line)
		}
	}
	return strings.TrimSpace(blankLinesRe.ReplaceAllString(strings.Join(kept, "\n"), "\n\n"))
}
