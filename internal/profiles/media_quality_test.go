package profiles

import (
	"testing"

	"github.com/yeixio/toskar-core/pkg/contracts"
)

func TestMediaQualityIsValidated(t *testing.T) {
	for _, q := range []string{"", "fast", "balanced", "best"} {
		if err := ValidateOrchestration(contracts.OrchestrationPolicy{MediaQuality: q}); err != nil {
			t.Errorf("%q refused: %v", q, err)
		}
	}
	if err := ValidateOrchestration(contracts.OrchestrationPolicy{MediaQuality: "ultra"}); err == nil {
		t.Error("ultra accepted")
	}
}
