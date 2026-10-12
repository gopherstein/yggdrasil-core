package imagegen

// A picture or clip can come out smaller or shorter than asked: clips are
// at most maxSeconds long, and each model makes only so many pixels. The
// result says so (#529), and the chat tells the person, so nothing is cut
// back silently.

// limits is what a job made less of than asked, or nil: the size asked
// when the picture or clip is clearly smaller, and the length asked when a
// clip is clearly shorter, with the most a clip can be.
func limits(askedW, askedH, madeW, madeH int, askedSeconds, madeSeconds float64) map[string]any {
	out := map[string]any{}
	if askedW > 0 && askedH > 0 && float64(madeW*madeH) < 0.8*float64(askedW*askedH) {
		out["asked_width"], out["asked_height"] = askedW, askedH
		out["made_width"], out["made_height"] = madeW, madeH
	}
	if askedSeconds > 0 && madeSeconds > 0 && madeSeconds < askedSeconds-0.5 {
		out["asked_seconds"], out["made_seconds"], out["max_seconds"] = askedSeconds, madeSeconds, float64(maxSeconds)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
