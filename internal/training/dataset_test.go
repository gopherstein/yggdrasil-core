package training

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestParseExamplesFormats(t *testing.T) {
	cases := []struct {
		name, file, text string
		want             int
		firstUser        string
	}{
		{"chat jsonl", "a.jsonl", `{"messages":[{"role":"user","content":"hi"},{"role":"assistant","content":"hello there"}]}` + "\n" +
			`{"messages":[{"role":"user","content":"bye"},{"role":"assistant","content":"see you"}]}`, 2, "hi"},
		{"sharegpt", "a.json", `[{"conversations":[{"from":"human","value":"q1"},{"from":"gpt","value":"a1 ok"}]}]`, 1, "q1"},
		{"prompt completion", "a.jsonl", `{"prompt":"What fits a 2019 Civic?","completion":"Which trim is it?"}`, 1, "What fits a 2019 Civic?"},
		{"alpaca", "a.jsonl", `{"instruction":"Classify the request","input":"flat tire on I-5","output":"roadside"}`, 1, "Classify the request\n\nflat tire on I-5"},
		{"qa csv", "faq.csv", "question,answer\nDo you rotate tires?,Yes with every oil change.\n", 1, "Do you rotate tires?"},
		{"catalog csv", "stock.csv", "sku,price\nA1,9.99\n", 0, ""},
		{"pasted QA", "notes.txt", "Q: What PSI?\nA: Check the door jamb sticker.\n\nQ: Can I mix brands?\nA: Match all four when you can.", 2, "What PSI?"},
		{"pasted dialog", "chat.txt", "Customer: I need tires\nAgent: What is the year, make, and model?\nCustomer: 2018 Camry\nAgent: Great, 215/55R17 fits.", 2, "I need tires"},
		{"prose", "policy.md", "# Returns\n\nUnmounted tires can be returned within 30 days.", 0, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseExamples(tc.file, tc.text)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != tc.want {
				t.Fatalf("got %d examples: %+v", len(got), got)
			}
			if tc.want > 0 && got[0][0].Content != tc.firstUser {
				t.Fatalf("first user turn = %q", got[0][0].Content)
			}
		})
	}
}

func TestParseExamplesReportsBadJSONLines(t *testing.T) {
	_, err := ParseExamples("x.jsonl", "{\"prompt\":\"a\",\"completion\":\"b\"}\nnot json\n")
	if err == nil || !strings.Contains(err.Error(), "line 2") {
		t.Fatalf("want a line number, got %v", err)
	}
}

func ex(user, answer string) Example {
	return Example{Messages: []Message{{Role: "user", Content: user}, {Role: "assistant", Content: answer}}}
}

func TestValidateExamplesFlags(t *testing.T) {
	examples := []Example{
		ex("What PSI?", "Check the sticker in the door jamb."),
		ex("what  psi?", "check the sticker in the door jamb."), // duplicate after normalizing
		ex("", ""),
		{Messages: []Message{{Role: "user", Content: "unanswered"}}},
		ex("Price of MP-22545?", "It is $189.99 and we have 12 in stock."),
		ex("Thanks", "Welcome"),
		ex("Any in stock?", "Let me check what is in stock for your size."),
		ex(strings.Repeat("long ", 3000), "ok then"),
	}
	got := ValidateExamples(examples, 2048)
	want := [][]Flag{nil, {FlagDuplicate}, {FlagEmpty}, {FlagNoAnswer}, {FlagVolatile}, {FlagShort}, nil, {FlagTooLong}}
	for i := range want {
		if fmt.Sprint(got[i].Flags) != fmt.Sprint(want[i]) {
			t.Errorf("example %d flags = %v, want %v", i, got[i].Flags, want[i])
		}
	}
	usable := 0
	for _, e := range got {
		if e.Usable() {
			usable++
		}
	}
	// The original, the volatile one, the short one, and the stock question still train.
	if usable != 4 {
		t.Fatalf("usable = %d", usable)
	}
}

func TestExcludedExampleDoesNotHideALaterCopy(t *testing.T) {
	a := ex("What PSI?", "Check the door sticker.")
	a.Excluded = true
	got := ValidateExamples([]Example{a, ex("What PSI?", "Check the door sticker.")}, 0)
	if len(got[1].Flags) != 0 || !got[1].Usable() {
		t.Fatalf("second copy flags = %v", got[1].Flags)
	}
}

func TestStatsWarnsOnSmallAndVolatileDatasets(t *testing.T) {
	var examples []Example
	for i := 0; i < 5; i++ {
		examples = append(examples, ex(fmt.Sprintf("q%d please", i), "It costs $10.00 today."))
	}
	st := Stats(ValidateExamples(examples, 0))
	if st.Usable != 5 || len(st.Warnings) != 2 {
		t.Fatalf("stats = %+v", st)
	}
	if !strings.Contains(st.Warnings[0], "at least 10") || !strings.Contains(st.Warnings[1], "connected knowledge") {
		t.Fatalf("warnings = %v", st.Warnings)
	}
}

func TestSplitHoldsOutATenthAndIsStable(t *testing.T) {
	var examples []Example
	for i := 0; i < 40; i++ {
		examples = append(examples, ex(fmt.Sprintf("question %d", i), fmt.Sprintf("answer number %d", i)))
	}
	train, valid := Split(examples)
	if len(train) != 36 || len(valid) != 4 {
		t.Fatalf("split %d/%d", len(train), len(valid))
	}
	// Reordering the input does not change the split.
	rev := append([]Example(nil), examples...)
	for i, j := 0, len(rev)-1; i < j; i, j = i+1, j-1 {
		rev[i], rev[j] = rev[j], rev[i]
	}
	_, valid2 := Split(rev)
	if fmt.Sprint(valid) != fmt.Sprint(valid2) {
		t.Fatal("split changed with input order")
	}
	tiny, tinyValid := Split(examples[:2])
	if len(tiny) != 2 || len(tinyValid) != 1 {
		t.Fatalf("tiny split %d/%d", len(tiny), len(tinyValid))
	}
}

func TestWriteJSONLAddsInstructions(t *testing.T) {
	out := WriteJSONL([]Example{ex("hi", "hello there")}, "You are Tire Bot.")
	var rec struct{ Messages []Message }
	if err := json.Unmarshal(out[:len(out)-1], &rec); err != nil {
		t.Fatal(err)
	}
	if len(rec.Messages) != 3 || rec.Messages[0].Role != "system" || rec.Messages[0].Content != "You are Tire Bot." {
		t.Fatalf("messages = %+v", rec.Messages)
	}
	// An example with its own system turn keeps it.
	own := Example{Messages: []Message{{Role: "system", Content: "custom"}, {Role: "user", Content: "a"}, {Role: "assistant", Content: "b c"}}}
	if got := WithInstructions(own.Messages, "You are Tire Bot."); got[0].Content != "custom" || len(got) != 3 {
		t.Fatalf("got %+v", got)
	}
}

func TestClassifyMaterial(t *testing.T) {
	cases := []struct {
		name, file, text string
		want             Use
		canTrain         bool
		signal           string
	}{
		{"inventory table", "inventory.csv", "sku,brand,size,price,in_stock\nMP-1,Michelin,225/45R17,189.99,12\n", UseKnowledge, false, "changing_columns"},
		{"static table", "glossary.csv", "term,meaning\nPSI,pounds per square inch\n", UseKnowledge, false, ""},
		{"chat examples", "chats.jsonl", `{"messages":[{"role":"user","content":"I need tires"},{"role":"assistant","content":"What year, make, and model?"}]}`, UseTraining, true, ""},
		{"examples quoting prices", "chats.jsonl",
			`{"prompt":"Price of MP-1?","completion":"It is $189.99."}` + "\n" + `{"prompt":"Stock of MP-2?","completion":"We have 4 left."}`, UseBoth, true, "changing_facts"},
		{"policy document", "returns.md", "# Returns\n\nUpdated 2026-01-05. Returns within 30 days.", UseKnowledge, false, "changing_facts"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := Classify(tc.file, tc.text)
			if rec.Use != tc.want || rec.CanTrain != tc.canTrain || len(rec.Reasons) == 0 {
				t.Fatalf("got %+v", rec)
			}
			if tc.signal != "" {
				found := false
				for _, s := range rec.Signals {
					found = found || s.Kind == tc.signal
				}
				if !found {
					t.Fatalf("missing signal %s in %+v", tc.signal, rec.Signals)
				}
			}
		})
	}
}

func TestChoiceWarning(t *testing.T) {
	inventory := Classify("inventory.csv", "sku,price\nA1,9.99\n")
	if _, err := ChoiceWarning(inventory, UseTraining); err == nil {
		t.Fatal("training on a table with no examples must be refused")
	}
	chats := Classify("c.jsonl", `{"prompt":"hi","completion":"hello there"}`)
	if w, err := ChoiceWarning(chats, UseTraining); err != nil || w != "" {
		t.Fatalf("matching choice warned: %q %v", w, err)
	}
	if w, _ := ChoiceWarning(chats, UseKnowledge); !strings.Contains(w, "will not change how the AI responds") {
		t.Fatalf("warning = %q", w)
	}
	priced := Classify("c.jsonl", `{"prompt":"Price?","completion":"It is $5.00."}`)
	if w, _ := ChoiceWarning(priced, UseTraining); !strings.Contains(w, "Connect the source data") {
		t.Fatalf("warning = %q", w)
	}
	// A Q&A table the user insists on training from a changing-data file.
	qaPrices := Recommendation{Use: UseKnowledge, CanTrain: true}
	if w, _ := ChoiceWarning(qaPrices, UseTraining); !strings.Contains(w, "old values") {
		t.Fatalf("warning = %q", w)
	}
}
