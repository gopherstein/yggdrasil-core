package automations

import (
	"context"
	"strings"
	"testing"
)

func TestNotifyCommandKeepsUserTextOutOfTheProgram(t *testing.T) {
	notice := Notice{Title: `$(calc)`, Body: "price is $420"}
	_, args, env, err := notifyCommand("windows", notice)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.Join(args, "\n"), notice.Title) {
		t.Fatal("title was interpolated into the PowerShell program")
	}
	joined := strings.Join(env, "\n")
	if !strings.Contains(joined, notice.Title) || !strings.Contains(joined, notice.Body) {
		t.Fatalf("env = %v", env)
	}

	_, args, _, err = notifyCommand("darwin", notice)
	if err != nil {
		t.Fatal(err)
	}
	if len(args) < 4 || args[2] != notice.Title || args[3] != notice.Body {
		t.Fatalf("darwin args = %#v", args)
	}
	if strings.Contains(args[1], notice.Title) {
		t.Fatal("title was interpolated into the osascript program")
	}

	name, args, _, err := notifyCommand("linux", notice)
	if err != nil || name != "notify-send" || args[len(args)-1] != notice.Body {
		t.Fatalf("linux %s %#v err=%v", name, args, err)
	}
}

func TestOSSenderUsesTheInjectedRunner(t *testing.T) {
	var called bool
	sender := OSSender{Run: func(context.Context, string, []string, []string) error {
		called = true
		return nil
	}}
	if err := sender.Notify(context.Background(), Notice{Title: "Morning price", Body: "The laptop is $420."}); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("expected the platform command to be handed to Run")
	}
}
