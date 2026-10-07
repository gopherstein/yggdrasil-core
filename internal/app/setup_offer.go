package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/yeixio/toskar-core/internal/inventory"
	"github.com/yeixio/toskar-core/internal/profiles"
	"github.com/yeixio/toskar-core/internal/structured"
	"github.com/yeixio/toskar-core/internal/tools"
	"github.com/yeixio/toskar-core/pkg/contracts"
	"github.com/yeixio/toskar-core/pkg/pluginapi"
)

// setupOptions lists what Yggdrasil can install to add a missing ability,
// with its size and the computer it would run on (Gungnir §28–29). Image
// generation is set up on this computer.
func (a *App) setupOptions(s inventory.Snapshot) []inventory.Setup {
	var out []inventory.Setup
	local := inventory.Node{}
	for _, n := range s.Nodes {
		if n.Local {
			local = n
			break
		}
	}
	if a.Images != nil {
		if st := a.Images.Status(); st.Supported && !st.Ready {
			for _, m := range st.Models {
				// A model this computer hasn't the memory for isn't offered;
				// the tool says why instead.
				if m.Recommended && !m.TooLittleMemory {
					out = append(out, inventory.Setup{Ability: "image_generation", Option: m.ID, Name: m.Name, SizeBytes: m.SizeBytes,
						NodeID: local.ID, NodeName: local.Name, Tools: []string{"image.generate", "image.edit"}, Slow: !st.Accelerated, TightMemory: m.TightMemory})
				}
			}
		}
	}
	if a.Video != nil {
		if st := a.Video.Status(); st.Supported && !st.Ready {
			for _, m := range st.Models {
				if m.Recommended && !m.TooLittleMemory {
					out = append(out, inventory.Setup{Ability: "video_generation", Option: m.ID, Name: m.Name, SizeBytes: m.SizeBytes,
						NodeID: local.ID, NodeName: local.Name, Tools: []string{"video.generate"}, Slow: !st.Accelerated, TightMemory: m.TightMemory})
				}
			}
		}
	}
	return out
}

func setupOffer(a inventory.Ability, request string) *contracts.SetupOffer {
	s := a.Setup
	return &contracts.SetupOffer{Ability: a.ID, Label: a.Label, Option: s.Option, Name: s.Name, SizeBytes: s.SizeBytes,
		NodeID: s.NodeID, NodeName: s.NodeName, Request: request, Slow: s.Slow, TightMemory: s.TightMemory}
}

// offerSetup answers a request for an ability that is not installed but can
// be, such as "Make me an image of a Viking tree" before image generation is
// set up: it says what would be installed, how large it is, and where, and
// the app offers to install it and then finish the request (Gungnir §29).
// Only chats in the app get the offer; an API caller gets the model's
// answer as before.
func (a *App) offerSetup(ctx context.Context, profile profiles.Profile, conversationID, message string) (<-chan pluginapi.ChatChunk, bool) {
	if conversationID == "" || len(structured.SchemaFrom(ctx)) > 0 {
		return nil, false
	}
	need, ok := inventory.Needs(a.Capabilities(ctx), message)
	if !ok {
		return nil, false
	}
	// A profile that keeps these tools out is not offered them.
	allowed := false
	for _, id := range need.Setup.Tools {
		if !strings.EqualFold(tools.PolicyForProfile(profile, id), tools.PolicyDeny) {
			allowed = true
		}
	}
	if !allowed {
		return nil, false
	}
	offer := setupOffer(need, message)
	where := "on this computer"
	if offer.NodeName != "" {
		where = "on this computer (" + offer.NodeName + ")"
	}
	label := strings.ToLower(need.Label[:1]) + need.Label[1:]
	reply := fmt.Sprintf("I can't %s yet, but I can set it up %s: %s, %.1f GB to download. Set it up below, and I'll finish this once it's ready.",
		label, where, offer.Name, float64(offer.SizeBytes)/1e9)
	// Say before it's set up if it will be slow or may fail here.
	if offer.TightMemory {
		reply += " This computer has less memory than it's comfortable with, so it may be slow or fail."
	}
	if offer.Slow {
		if need.ID == "video_generation" {
			reply += " This computer has no GPU acceleration for it, so a clip can take most of an hour."
		} else {
			reply += " This computer has no GPU acceleration for it, so each picture takes a few minutes."
		}
	}
	meta := &contracts.MessageMeta{Setup: offer,
		Steps: []contracts.ActivityStep{{Kind: "share", Text: "Found a way to " + label}}}
	return a.replyDirectly(ctx, conversationID, message, reply, meta), true
}

// mediaSlow reports whether making a picture (kind image or edit) or a
// clip (video) here takes minutes: no GPU acceleration, or less memory
// than its model is comfortable with.
func (a *App) mediaSlow(kind string) bool {
	setup := a.Images
	if kind == "video" {
		setup = a.Video
	}
	if setup == nil {
		return false
	}
	if !setup.Accelerated() {
		return true
	}
	for _, m := range setup.Status().Models {
		if m.ID == setup.ActiveModel() && m.TightMemory {
			return true
		}
	}
	return false
}
