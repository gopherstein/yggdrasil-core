package app

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/yeixio/yggdrasil-core/internal/config"
	"github.com/yeixio/yggdrasil-core/internal/egress"
	"github.com/yeixio/yggdrasil-core/internal/ratings"
	"github.com/yeixio/yggdrasil-core/internal/version"
	"github.com/yeixio/yggdrasil-core/pkg/contracts"
)

// newRatings sets up community model ratings (#37). Ratings stay on this
// computer unless the person shares one; each rating shared or withdrawn,
// and each download of the public summary, is recorded in What left this
// computer.
func (a *App) newRatings(cfg config.Config) *ratings.Service {
	var (
		mu  sync.Mutex
		inv contracts.HardwareInventory
		at  time.Time
	)
	return &ratings.Service{
		DB:       a.DB.SQL,
		Settings: a.Settings,
		Client: &ratings.Client{
			ServiceURL: cfg.RatingsURL, SummaryURL: cfg.RatingsSummaryURL,
			HTTP:      &http.Client{Timeout: 20 * time.Second},
			UserAgent: "Yggdrasil/" + version.Version + " (+https://github.com/yeixio/yggdrasil-core)",
		},
		// Hardware detection is slow on some computers, and it is the same
		// from one minute to the next.
		Hardware: func(ctx context.Context) (contracts.HardwareInventory, error) {
			mu.Lock()
			defer mu.Unlock()
			if time.Since(at) < 10*time.Minute {
				return inv, nil
			}
			got, err := a.detectHardware(ctx)
			if err != nil {
				return got, err
			}
			inv, at = got, time.Now()
			return inv, nil
		},
		Models: a.Models.List,
		Record: func(ctx context.Context, destination, detail string) {
			if a.Egress != nil {
				a.Egress.Add(ctx, egress.CommunityRatings, destination, detail)
			}
		},
		AppVersion: version.Version,
	}
}
