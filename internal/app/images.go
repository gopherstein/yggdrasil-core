package app

import (
	"context"
	"path/filepath"
	"sync"
	"time"

	"github.com/yeixio/toskar-core/internal/config"
	"github.com/yeixio/toskar-core/internal/imagegen"
	"github.com/yeixio/toskar-core/internal/runtimes/llamacpp"
)

// newImageSetup keeps stable-diffusion.cpp with the runtimes and image
// models with the models, and sizes its recommendation to this computer.
func (a *App) newImageSetup(cfg config.Config) *imagegen.Setup {
	return a.newSDSetup(cfg, "images", nil)
}

// newVideoSetup is video generation's setup: the same stable-diffusion.cpp,
// with video models in their own folder (Gungnir §27).
func (a *App) newVideoSetup(cfg config.Config) *imagegen.Setup {
	return a.newSDSetup(cfg, "video", imagegen.VideoCatalog)
}

func (a *App) newSDSetup(cfg config.Config, folder string, catalog func() []imagegen.Model) *imagegen.Setup {
	memory := sync.OnceValue(func() int64 {
		inv, err := a.hw.Detect(context.Background())
		if err != nil {
			return 0
		}
		return int64(inv.Memory.TotalBytes)
	})
	return &imagegen.Setup{
		ProgramDir: filepath.Join(cfg.RuntimesDir, "sdcpp"),
		ModelsDir:  filepath.Join(cfg.ModelsDir, folder),
		Catalog:    catalog,
		Memory:     memory,
		FreeBytes: func() (int64, error) {
			inv, err := a.hw.Detect(context.Background())
			if err != nil {
				return 0, err
			}
			return int64(inv.Disk.AvailableBytes), nil
		},
		Sandboxed: a.python.Sandboxed,
		// The Vulkan build when a GPU can run it (#154), checked once: it
		// runs vulkaninfo, and the answer doesn't change while running.
		GPU: vulkanUsable,
		// "Can you make images?" and setup offers read the inventory.
		Changed: a.invalidateCapabilities,
	}
}

// vulkanUsable reports, once, whether a Vulkan build would use a GPU here;
// images, video, and llama.cpp agree on it.
var vulkanUsable = sync.OnceValue(func() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	return llamacpp.VulkanUsable(ctx)
})
