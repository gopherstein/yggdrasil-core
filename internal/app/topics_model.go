package app

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/yeixio/toskar-core/internal/runlog"
	"github.com/yeixio/toskar-core/pkg/pluginapi"
)

// The topic checks run on a small model that holds them well (#457): the
// catalog tags the models that pass tests/quality/topics.json, and the
// smallest one installed here is used when it's the answering model,
// already running, or fits in memory beside it. Otherwise the checks run
// on the answering model.

// TopicCheckTag marks a catalog model verified for the topic checks.
const TopicCheckTag = "topic-check"

// topicCheckRole is how the check model's calls appear in the run trace.
const topicCheckRole = "topic check"

// verifiedForTopics reports a catalog model that passes the topic cases.
func (a *App) verifiedForTopics(modelID string) bool {
	if a.Models == nil {
		return false
	}
	entry, ok := a.Models.Catalog().Get(modelID)
	return ok && slices.Contains(entry.Tags, TopicCheckTag)
}

// topicCheckModel is the model the checks run on beside answering, or
// answering itself.
func (a *App) topicCheckModel(ctx context.Context, answering string) string {
	if a.Models == nil {
		return answering
	}
	running := map[string]bool{}
	if views, err := a.localRunningViews(ctx); err == nil {
		for _, v := range views {
			running[v.ModelID] = true
		}
	}
	var answerNeeds uint64
	if entry, ok := a.Models.Catalog().Get(answering); ok {
		answerNeeds = entry.MemoryNeededBytes
	}
	var verified []checkCandidate
	for _, m := range a.installedModels(ctx) {
		if m.Installed && a.verifiedForTopics(m.ID) {
			verified = append(verified, checkCandidate{id: m.ID, needs: m.MemoryNeeded, running: running[m.ID]})
		}
	}
	return pickCheckModel(answering, answerNeeds, a.memoryTotal(ctx), verified)
}

// checkCandidate is an installed model verified for the topic checks.
type checkCandidate struct {
	id      string
	needs   uint64
	running bool
}

// pickCheckModel is the smallest verified model that is the answering
// model, already running, or fits beside it with a quarter of memory to
// spare; else the answering model. total 0 is memory not known.
func pickCheckModel(answering string, answerNeeds, total uint64, verified []checkCandidate) string {
	best, bestNeeds := "", uint64(0)
	for _, m := range verified {
		fits := m.id == answering || m.running || total == 0 || m.needs+answerNeeds <= total*3/4
		if fits && (best == "" || m.needs < bestNeeds) {
			best, bestNeeds = m.id, m.needs
		}
	}
	if best == "" {
		return answering
	}
	return best
}

// checkText runs one topic check call: on the check model here when it
// isn't the answering model, else as the turn's own call.
func (e *chatExecEnv) checkText(ctx context.Context, role string, messages []pluginapi.ChatMessage) (string, error) {
	answering := e.modelForRole(role)
	checker := e.app.topicCheckModel(ctx, answering)
	if checker == "" || checker == answering {
		return collectText(ctx, e, role, messages)
	}
	e.noteCheckModel(ctx, checker)
	local := e.app.Config.Get().NodeID
	started := time.Now()
	ch, err := e.app.generateOnNode(ctx, local, checker, topicCheckRole, "", messages)
	if err != nil {
		// The answering model checks instead.
		return collectText(ctx, e, role, messages)
	}
	ch = traceGeneration(runlog.From(ctx), ch, checker, topicCheckRole, e.app.nodeDisplayName(local), started, e.app.localAcceleration(ctx, checker))
	var b strings.Builder
	for chunk := range ch {
		if chunk.Error != "" {
			return b.String(), errString(chunk.Error)
		}
		b.WriteString(chunk.Content)
	}
	return b.String(), nil
}

// noteCheckModel says once in the run trace which model checked the topic.
func (e *chatExecEnv) noteCheckModel(ctx context.Context, modelID string) {
	e.mu.Lock()
	noted := e.checkModelNoted
	e.checkModelNoted = true
	e.mu.Unlock()
	if !noted {
		runlog.From(ctx).Note("topicCheckModel", map[string]any{"model": e.app.modelName(modelID)})
	}
}
