package training

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

// ParseExamples reads training examples from material. It understands:
//
//   - JSONL or a JSON array of {"messages": [...]} chats
//   - JSONL of prompt/completion, input/output, question/answer, or
//     instruction/input/output records
//   - CSV/TSV with a question-like and an answer-like column
//   - pasted text of "Q:"/"A:" or "User:"/"Assistant:" blocks
//
// It returns no examples, and no error, for material that is not examples,
// such as a product catalog or a policy document.
func ParseExamples(filename, text string) ([][]Message, error) {
	text = strings.TrimPrefix(strings.ReplaceAll(text, "\r\n", "\n"), "\ufeff")
	ext := strings.ToLower(filepath.Ext(filename))
	trimmed := strings.TrimSpace(text)
	switch {
	case ext == ".csv":
		return examplesFromTable(text, ',')
	case ext == ".tsv":
		return examplesFromTable(text, '\t')
	case ext == ".jsonl" || ext == ".json" || strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "["):
		return examplesFromJSON(trimmed)
	default:
		return examplesFromDialog(text), nil
	}
}

var (
	promptKeys  = []string{"instruction", "prompt", "question", "input", "user", "query", "request"}
	answerKeys  = []string{"completion", "answer", "output", "response", "assistant", "reply", "target"}
	contextKeys = []string{"context", "input"}
)

func examplesFromJSON(text string) ([][]Message, error) {
	var records []map[string]any
	if strings.HasPrefix(text, "[") {
		if err := json.Unmarshal([]byte(text), &records); err != nil {
			return nil, fmt.Errorf("not a JSON array of objects: %w", err)
		}
	} else {
		sc := bufio.NewScanner(strings.NewReader(text))
		sc.Buffer(make([]byte, 64*1024), 8<<20)
		line := 0
		for sc.Scan() {
			line++
			raw := strings.TrimSpace(sc.Text())
			if raw == "" {
				continue
			}
			var rec map[string]any
			if err := json.Unmarshal([]byte(raw), &rec); err != nil {
				return nil, fmt.Errorf("line %d is not a JSON object", line)
			}
			records = append(records, rec)
		}
		if err := sc.Err(); err != nil {
			return nil, err
		}
	}
	var out [][]Message
	for _, rec := range records {
		if msgs, ok := chatRecord(rec); ok {
			out = append(out, msgs)
			continue
		}
		if msgs, ok := pairRecord(rec); ok {
			out = append(out, msgs)
		}
	}
	return out, nil
}

func chatRecord(rec map[string]any) ([]Message, bool) {
	raw, ok := rec["messages"]
	if !ok {
		raw = rec["conversations"]
	}
	list, ok := raw.([]any)
	if !ok {
		return nil, false
	}
	var msgs []Message
	for _, item := range list {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		role, _ := m["role"].(string)
		if role == "" {
			role, _ = m["from"].(string) // ShareGPT
		}
		content, _ := m["content"].(string)
		if content == "" {
			content, _ = m["value"].(string)
		}
		msgs = append(msgs, Message{Role: normalizeRole(role), Content: content})
	}
	return msgs, true
}

func pairRecord(rec map[string]any) ([]Message, bool) {
	get := func(keys []string, skip string) (string, string) {
		for _, k := range keys {
			if k == skip {
				continue
			}
			for rk, v := range rec {
				if strings.EqualFold(rk, k) {
					if s, ok := v.(string); ok {
						return rk, s
					}
				}
			}
		}
		return "", ""
	}
	pk, prompt := get(promptKeys, "")
	_, answer := get(answerKeys, "")
	if pk == "" || prompt == "" && answer == "" {
		return nil, false
	}
	// Alpaca: instruction plus optional input.
	if strings.EqualFold(pk, "instruction") {
		if _, extra := get(contextKeys, "instruction"); strings.TrimSpace(extra) != "" {
			prompt = strings.TrimSpace(prompt) + "\n\n" + strings.TrimSpace(extra)
		}
	}
	msgs := []Message{{Role: "user", Content: prompt}, {Role: "assistant", Content: answer}}
	if _, sys := get([]string{"system"}, ""); strings.TrimSpace(sys) != "" {
		msgs = append([]Message{{Role: "system", Content: sys}}, msgs...)
	}
	return msgs, true
}

func normalizeRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "user", "human", "customer", "q", "question":
		return "user"
	case "assistant", "gpt", "bot", "ai", "model", "agent", "a", "answer":
		return "assistant"
	case "system":
		return "system"
	default:
		return strings.ToLower(strings.TrimSpace(role))
	}
}

// examplesFromTable reads a table with one question-like and one answer-like
// column. Any other table is not examples.
func examplesFromTable(text string, sep rune) ([][]Message, error) {
	r := csv.NewReader(strings.NewReader(text))
	r.Comma = sep
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	rows, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) < 2 {
		return nil, nil
	}
	pi, ai := -1, -1
	for i, h := range rows[0] {
		h = strings.ToLower(strings.TrimSpace(h))
		if pi < 0 && containsWord(promptKeys, h) {
			pi = i
		} else if ai < 0 && containsWord(answerKeys, h) {
			ai = i
		}
	}
	if pi < 0 || ai < 0 {
		return nil, nil
	}
	var out [][]Message
	for _, row := range rows[1:] {
		if pi >= len(row) || ai >= len(row) {
			continue
		}
		out = append(out, []Message{{Role: "user", Content: row[pi]}, {Role: "assistant", Content: row[ai]}})
	}
	return out, nil
}

func containsWord(words []string, s string) bool {
	for _, w := range words {
		if s == w {
			return true
		}
	}
	return false
}

var turnRe = regexp.MustCompile(`(?im)^\s*(q|question|a|answer|user|customer|human|assistant|agent|bot|ai)\s*:\s?`)

// examplesFromDialog reads pasted "Q:/A:" or "User:/Assistant:" blocks. A new
// example starts at each user turn that follows an assistant turn, or at a
// blank-line-separated block.
func examplesFromDialog(text string) [][]Message {
	var out [][]Message
	for _, block := range regexp.MustCompile(`\n\s*\n\s*\n|\n-{3,}\n`).Split(text, -1) {
		locs := turnRe.FindAllStringSubmatchIndex(block, -1)
		if len(locs) < 2 {
			continue
		}
		var cur []Message
		for i, loc := range locs {
			role := normalizeRole(block[loc[2]:loc[3]])
			end := len(block)
			if i+1 < len(locs) {
				end = locs[i+1][0]
			}
			content := strings.TrimSpace(block[loc[1]:end])
			if role == "user" && len(cur) > 0 && cur[len(cur)-1].Role == "assistant" {
				out = append(out, cur)
				cur = nil
			}
			cur = append(cur, Message{Role: role, Content: content})
		}
		if len(cur) > 0 {
			out = append(out, cur)
		}
	}
	return out
}

// EstimateTokens is a tokenizer-free estimate: about four characters per
// token for English, with a floor for short words.
func EstimateTokens(s string) int {
	n := utf8.RuneCountInString(s)
	words := len(strings.Fields(s))
	est := n / 4
	if words > est {
		est = words
	}
	return est
}

func exampleTokens(msgs []Message) int {
	total := 0
	for _, m := range msgs {
		// Chat templates add role headers and separators around each turn.
		total += EstimateTokens(m.Content) + 8
	}
	return total
}

var volatileRe = regexp.MustCompile(`(?i)(\$\s?\d[\d,]*(\.\d{2})?|\b\d[\d,]*(\.\d{2})?\s?(usd|eur|gbp|dollars?)\b|\b(out of stock|\d+\s+(left|available|in stock|units?))\b|\bsku[:#\s-]*[a-z0-9-]{3,}\b)`)

// ContainsVolatileFacts reports whether text states prices, stock, or SKUs.
func ContainsVolatileFacts(text string) bool { return volatileRe.MatchString(text) }

// ValidateExamples flags problems in a set of examples, in order. The first
// copy of a duplicate is kept; later copies are flagged.
func ValidateExamples(examples []Example, maxSeqTokens int) []Example {
	if maxSeqTokens <= 0 {
		maxSeqTokens = 2048
	}
	seen := map[string]bool{}
	out := make([]Example, len(examples))
	for i, ex := range examples {
		ex.Flags = nil
		userText, answer := "", ""
		for _, m := range ex.Messages {
			switch m.Role {
			case "user":
				userText += strings.TrimSpace(m.Content)
			case "assistant":
				answer = strings.TrimSpace(m.Content)
			}
		}
		switch {
		case strings.TrimSpace(userText) == "" && answer == "":
			ex.Flags = append(ex.Flags, FlagEmpty)
		case len(ex.Messages) == 0 || ex.Messages[len(ex.Messages)-1].Role != "assistant" || answer == "":
			ex.Flags = append(ex.Flags, FlagNoAnswer)
		}
		if answer != "" && len(strings.Fields(answer)) < 2 {
			ex.Flags = append(ex.Flags, FlagShort)
		}
		if exampleTokens(ex.Messages) > maxSeqTokens {
			ex.Flags = append(ex.Flags, FlagTooLong)
		}
		if ContainsVolatileFacts(answer) {
			ex.Flags = append(ex.Flags, FlagVolatile)
		}
		key := exampleKey(ex.Messages)
		if key != "" && seen[key] {
			ex.Flags = append(ex.Flags, FlagDuplicate)
		} else if key != "" && !ex.Excluded {
			seen[key] = true
		}
		out[i] = ex
	}
	return out
}

var spaceRe = regexp.MustCompile(`\s+`)

// exampleKey normalizes case and whitespace so trivially different copies of
// the same example count as duplicates.
func exampleKey(msgs []Message) string {
	var b strings.Builder
	for _, m := range msgs {
		if m.Role == "system" {
			continue
		}
		b.WriteString(m.Role)
		b.WriteByte(0)
		b.WriteString(spaceRe.ReplaceAllString(strings.ToLower(strings.TrimSpace(m.Content)), " "))
		b.WriteByte(0)
	}
	if b.Len() == 0 {
		return ""
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

// DatasetStats summarizes the examples that will train.
type DatasetStats struct {
	Total    int          `json:"total"`
	Usable   int          `json:"usable"`
	Excluded int          `json:"excluded"`
	Flagged  map[Flag]int `json:"flagged"`
	Tokens   int          `json:"tokens"`
	// P95Tokens sizes the maximum sequence length.
	P95Tokens int `json:"p95_tokens"`
	// Warnings are human sentences about the dataset as a whole.
	Warnings []string `json:"warnings,omitempty"`
}

// MinExamples is the smallest dataset Yggdrasil trains on.
const MinExamples = 10

// RecommendedExamples is where results usually become reliable.
const RecommendedExamples = 50

// Stats summarizes validated examples.
func Stats(examples []Example) DatasetStats {
	st := DatasetStats{Total: len(examples), Flagged: map[Flag]int{}}
	var lengths []int
	volatile := 0
	for _, ex := range examples {
		for _, f := range ex.Flags {
			st.Flagged[f]++
		}
		if ex.Excluded {
			st.Excluded++
		}
		if !ex.Usable() {
			continue
		}
		st.Usable++
		n := exampleTokens(ex.Messages)
		st.Tokens += n
		lengths = append(lengths, n)
		for _, f := range ex.Flags {
			if f == FlagVolatile {
				volatile++
			}
		}
	}
	if len(lengths) > 0 {
		sort.Ints(lengths)
		idx := (len(lengths)*95+99)/100 - 1
		if idx < 0 {
			idx = 0
		}
		st.P95Tokens = lengths[idx]
	}
	switch {
	case st.Usable < MinExamples:
		st.Warnings = append(st.Warnings, fmt.Sprintf("Add at least %d usable examples before training. You have %d.", MinExamples, st.Usable))
	case st.Usable < RecommendedExamples:
		st.Warnings = append(st.Warnings, fmt.Sprintf("%d examples can teach a format or tone. About %d or more teach a workflow reliably.", st.Usable, RecommendedExamples))
	}
	if volatile > 0 {
		st.Warnings = append(st.Warnings, answersStating(volatile)+". The model may memorize values that later change. Keep that data in connected knowledge and write answers that show how to use it.")
	}
	return st
}

// WithInstructionTokens adjusts stats for the system instructions training
// prepends to every example, which the trainer's memory depends on.
func WithInstructionTokens(st DatasetStats, instructions string) DatasetStats {
	n := EstimateTokens(instructions)
	if n == 0 || st.Usable == 0 {
		return st
	}
	n += 8
	st.P95Tokens += n
	st.Tokens += n * st.Usable
	return st
}

// HashExamples fingerprints the usable examples a revision was trained on.
func HashExamples(examples []Example) string {
	h := sha256.New()
	for _, ex := range examples {
		if ex.Usable() {
			b, _ := json.Marshal(ex.Messages)
			h.Write(b)
			h.Write([]byte{'\n'})
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

// Split divides usable examples into training and validation sets. The
// validation set is about a tenth, at least one example, and never empty. A
// fixed seed keeps the split stable between revisions.
func Split(examples []Example) (train, valid []Example) {
	var usable []Example
	for _, ex := range examples {
		if ex.Usable() {
			usable = append(usable, ex)
		}
	}
	if len(usable) == 0 {
		return nil, nil
	}
	sort.SliceStable(usable, func(i, j int) bool { return exampleKey(usable[i].Messages) < exampleKey(usable[j].Messages) })
	n := len(usable) / 10
	if n < 1 {
		n = 1
	}
	if len(usable) < 4 {
		// Too few to hold any out. Validate on a training example.
		return usable, usable[:1]
	}
	return usable[n:], usable[:n]
}

// WithInstructions prepends the AI's system instructions to an example that
// has none, so training matches what the model sees in chat.
func WithInstructions(msgs []Message, instructions string) []Message {
	instructions = strings.TrimSpace(instructions)
	if instructions == "" || (len(msgs) > 0 && msgs[0].Role == "system") {
		return msgs
	}
	return append([]Message{{Role: "system", Content: instructions}}, msgs...)
}

// WriteJSONL renders examples in the chat format trainers read.
func WriteJSONL(examples []Example, instructions string) []byte {
	var buf bytes.Buffer
	for _, ex := range examples {
		b, _ := json.Marshal(map[string]any{"messages": WithInstructions(ex.Messages, instructions)})
		buf.Write(b)
		buf.WriteByte('\n')
	}
	return buf.Bytes()
}
