package lifecycle

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// StopFunc unloads a running model instance.
type StopFunc func(ctx context.Context, instanceID string) error

// ListFunc returns currently loaded instances with last-used times.
type ListFunc func(ctx context.Context) ([]Instance, error)

// SettingsFunc returns lifecycle mode and idle minutes.
type SettingsFunc func(ctx context.Context) (mode string, idleMinutes int, err error)

// Instance is a loaded model tracked for idle unload.
type Instance struct {
	InstanceID string
	ModelID    string
	LastUsed   time.Time
}

// Sweeper periodically unloads idle models when lifecycle is automatic.
type Sweeper struct {
	List     ListFunc
	Stop     StopFunc
	Settings SettingsFunc
	Logger   *slog.Logger
	Interval time.Duration
	Now      func() time.Time
	// OnUnload is called after an idle model is stopped.
	OnUnload func(inst Instance)

	mu     sync.Mutex
	cancel context.CancelFunc
}

// Start begins the background loop.
func (s *Sweeper) Start(parent context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(parent)
	s.cancel = cancel
	interval := s.Interval
	if interval <= 0 {
		interval = 30 * time.Second
	}
	now := s.Now
	if now == nil {
		now = time.Now
	}
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				s.tick(ctx, now())
			}
		}
	}()
}

// Halt stops the background loop.
func (s *Sweeper) Halt() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
}

func (s *Sweeper) tick(ctx context.Context, now time.Time) {
	if s.Settings == nil || s.List == nil || s.Stop == nil {
		return
	}
	mode, idleMin, err := s.Settings(ctx)
	if err != nil || mode != "automatic" || idleMin <= 0 {
		return
	}
	items, err := s.List(ctx)
	if err != nil {
		return
	}
	cutoff := now.Add(-time.Duration(idleMin) * time.Minute)
	for _, inst := range items {
		last := inst.LastUsed
		if last.IsZero() {
			continue
		}
		if last.After(cutoff) {
			continue
		}
		if err := s.Stop(ctx, inst.InstanceID); err != nil {
			if s.Logger != nil {
				s.Logger.Warn("idle unload failed", "model", inst.ModelID, "error", err)
			}
			continue
		}
		if s.Logger != nil {
			s.Logger.Info("unloaded idle model", "model", inst.ModelID, "instance", inst.InstanceID)
		}
		if s.OnUnload != nil {
			s.OnUnload(inst)
		}
	}
}

// TickOnce runs a single sweep (for tests).
func (s *Sweeper) TickOnce(ctx context.Context, now time.Time) {
	s.tick(ctx, now)
}
