package main

import (
	"embed"
	"fmt"
	"io"
)

// The packages install these same files, so the script a user prints and the
// script a package installs do not drift.
//
//go:embed completions/yggctl.bash completions/_yggctl completions/yggctl.fish
var completionFiles embed.FS

const completionUsage = `usage: yggctl completion <bash|zsh|fish>
  bash: eval "$(yggctl completion bash)"
  zsh:  yggctl completion zsh > ~/.zfunc/_yggctl   (with ~/.zfunc on fpath before compinit)
  fish: yggctl completion fish > ~/.config/fish/completions/yggctl.fish`

var completionPaths = map[string]string{
	"bash": "completions/yggctl.bash",
	"zsh":  "completions/_yggctl",
	"fish": "completions/yggctl.fish",
}

func completionCommand(args []string, out io.Writer) error {
	if len(args) != 1 {
		return fmt.Errorf("%s", completionUsage)
	}
	path, ok := completionPaths[args[0]]
	if !ok {
		return fmt.Errorf("%s", completionUsage)
	}
	script, err := completionFiles.ReadFile(path)
	if err != nil {
		return err
	}
	_, err = out.Write(script)
	return err
}
