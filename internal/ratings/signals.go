package ratings

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/yeixio/yggdrasil-core/internal/models"
)

// confidenceWeight is how much a cohort's ratings count in recommendations:
// early ratings half, a community's fully. Limited ones are never published.
var confidenceWeight = map[string]float64{"early": 0.5, "community": 1}

// Signals are community ratings as a recommendation signal, by local model
// ID: the narrowest published cohort of hardware like this computer's, or
// everyone's. They come from the summary already kept, so asking never
// sends anything; with community ratings off there are none.
func (s *Service) Signals(ctx context.Context) (map[string]models.CommunitySignal, error) {
	if on, err := s.Settings.GetBool(ctx, SettingShow, false); err != nil || !on {
		return nil, err
	}
	var body string
	err := s.DB.QueryRowContext(ctx, `SELECT body FROM ratings_summary WHERE id = 1`).Scan(&body)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var snap Snapshot
	if err := json.Unmarshal([]byte(body), &snap); err != nil {
		return nil, nil
	}
	inv, err := s.Hardware(ctx)
	if err != nil {
		return nil, err
	}
	h, hok := Normalize(inv)
	backend := Backend(inv)
	list, err := s.Models(ctx)
	if err != nil {
		return nil, err
	}
	out := map[string]models.CommunitySignal{}
	for _, m := range list {
		id, err := Identify(m, backend)
		if err != nil {
			continue
		}
		mc, ok := lookup(snap, id, h, hok)
		if !ok {
			continue
		}
		st, similar := mc.Similar, true
		if st == nil || confidenceWeight[st.Confidence] == 0 {
			st, similar = mc.Overall, false
		}
		if st == nil || confidenceWeight[st.Confidence] == 0 {
			continue
		}
		out[m.ID] = models.CommunitySignal{
			Signal:  (st.WeightedScore - snap.Prior) * confidenceWeight[st.Confidence],
			Score:   st.WeightedScore,
			Ratings: st.Ratings,
			Similar: similar,
		}
	}
	return out, nil
}
