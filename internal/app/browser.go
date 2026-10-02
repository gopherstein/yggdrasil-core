package app

import (
	"context"
	"path/filepath"

	"github.com/yeixio/yggdrasil-core/internal/browser"
	"github.com/yeixio/yggdrasil-core/internal/config"
	"github.com/yeixio/yggdrasil-core/internal/egress"
	"github.com/yeixio/yggdrasil-core/internal/pyenv"
)

// registerBrowser adds web pages in an isolated browser (Gungnir §26): each
// chat gets its own headless Chrome, Edge, Chromium, or Brave with a fresh
// profile, which cannot reach this computer or the local network. Each page
// it opens is recorded in What left this computer.
func (a *App) registerBrowser(cfg config.Config) {
	a.browser = &browser.Manager{
		WorkDir:   filepath.Join(cfg.DataDir, "browser"),
		Guard:     &browser.Guard{},
		Sandboxed: pyenv.Sandboxed(),
		Opened: func(ctx context.Context, host, url string) {
			if a.Egress != nil {
				a.Egress.Add(ctx, egress.WebPage, host, url)
			}
		},
	}
	for _, t := range browser.Tools(a.browser, a.Artifacts) {
		a.Tools.Register(t)
	}
}
