package models

// CommunitySignal is what community ratings (#37) say about a model on
// hardware like this computer's. It is one input to recommendations, next
// to the curated order and fit, never the only one.
type CommunitySignal struct {
	// Signal is the confidence-weighted score less the average everyone's
	// ratings lean toward: above 0 is rated better than average, below 0
	// worse. Early ratings count half.
	Signal float64
	// Score and Ratings are the cohort's weighted score and rating count.
	Score   float64
	Ratings int
	// Similar is true when the ratings come from hardware like this
	// computer's rather than from everyone.
	Similar bool
}

// positionWeight is what one place in a curated list is worth against a
// community signal: a model two places down needs to be rated 0.6 better
// to be chosen first.
const positionWeight = 0.3

// pickByCommunity chooses among candidates in curated order: the one whose
// community signal, less positionWeight for each place down the list, is
// highest. With no ratings it is the first. It reports whether ratings
// changed the choice.
func pickByCommunity(ids []string, community map[string]CommunitySignal) (int, bool) {
	if len(ids) == 0 {
		return -1, false
	}
	best, bestValue := 0, community[ids[0]].Signal
	for i := 1; i < len(ids); i++ {
		if v := community[ids[i]].Signal - positionWeight*float64(i); v > bestValue {
			best, bestValue = i, v
		}
	}
	return best, best != 0
}
