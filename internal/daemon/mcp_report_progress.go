package daemon

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
)

// mcpReportProgress stores the agent's structured summary and tells the UIs.
func (d *Daemon) mcpReportProgress(ctx context.Context, s *session, raw json.RawMessage) (string, error) {
	var a protocol.ReportProgressArgs
	if err := decodeArgs(raw, &a); err != nil {
		return "", err
	}
	sum := core.Summary{
		SessionID:     s.id,
		Done:          clipList(a.Done, maxItem),
		FilesChanged:  clipList(a.FilesChanged, maxItem),
		Decisions:     clipList(a.Decisions, maxItem),
		OpenQuestions: clipList(a.OpenQuestions, maxItem),
		Next:          clipText(a.Next, maxItem),
		At:            d.now(),
	}
	for i, f := range sum.FilesChanged {
		sum.FilesChanged[i] = strings.ReplaceAll(f, "\\", "/")
	}
	if len(sum.Done) == 0 && len(sum.OpenQuestions) == 0 && len(sum.Decisions) == 0 && sum.Next == "" {
		return "", badRequest("report_progress needs at least one of done, decisions, open_questions or next")
	}
	if err := d.store.AddSummary(ctx, sum); err != nil {
		return "", err
	}
	d.publish(protocol.TopicSummary, sum)
	return "recorded", nil
}
