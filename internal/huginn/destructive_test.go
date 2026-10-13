package huginn

import (
	"strings"
	"testing"
)

func TestDestructiveCommand(t *testing.T) {
	for _, s := range []string{
		"run `rm -rf ~/Downloads/*`", "rm -r old", "rm --recursive x", "del /s /q C:\\Users\\me\\Downloads",
		"rmdir /S folder", "Remove-Item ~/Downloads/* -Recurse -Force", "find ~/Downloads -type f -delete",
		"sudo mkfs.ext4 /dev/sdb1", "dd if=/dev/zero of=/dev/sda", "format c:", "chmod -R 777 /",
		"git push --force origin main", "git push -f", "git reset --hard HEAD~3", "git clean -fdx",
	} {
		if DestructiveCommand(s) == "" {
			t.Errorf("not caught: %s", s)
		}
	}
	for _, s := range []string{
		"I did not delete anything, because you did not approve the command.", "Use rm to remove one file you name.",
		"The format of the date is ISO 8601.", "Find the file and delete it from the Trash.", "chmod 644 notes.txt",
		"git push origin main", "git reset --soft HEAD~1",
	} {
		if c := DestructiveCommand(s); c != "" {
			t.Errorf("caught %q in: %s", c, s)
		}
	}
}

func TestAsksToDo(t *testing.T) {
	for _, s := range []string{
		"Run a command to delete everything in my Downloads folder.",
		"Delete all the files in ~/tmp",
		"Can you wipe my old backups?",
		"Please clear out the Downloads folder",
		"Go ahead and force-push my branch",
		"I want you to empty the Trash",
		"Clean up my Downloads folder for me",
	} {
		if !AsksToDo(s) {
			t.Errorf("not a request to do it: %s", s)
		}
	}
	for _, s := range []string{
		"How do I delete everything in a folder from the terminal?",
		"What's the command to delete a folder and everything in it?",
		"Explain what rm -rf does",
		"Which command removes a git branch?",
		"Tell me how to wipe a USB drive",
		"What is the capital of France?",
		"Write a poem about autumn",
	} {
		if AsksToDo(s) {
			t.Errorf("taken as a request to do it: %s", s)
		}
	}
}

func TestWithoutDestructive(t *testing.T) {
	answer := "I can't do that directly. If you want to do it yourself, you can run:\n\n```bash\nrm -rf ~/Downloads/*\n```\n\nOr use `rm -rf ~/Downloads/*` in a terminal.\nrm -r ~/Downloads\n\nWant me to guide you? Keep `ls ~/Downloads` to look first."
	got := WithoutDestructive(answer)
	if DestructiveCommand(got) != "" {
		t.Fatalf("still has a command: %q", got)
	}
	if !strings.Contains(got, "I can't do that directly.") || !strings.Contains(got, "`ls ~/Downloads`") {
		t.Fatalf("lost the rest: %q", got)
	}
	if strings.Contains(got, "\n\n\n") {
		t.Fatalf("left blank lines: %q", got)
	}
}
