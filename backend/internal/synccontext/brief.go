package synccontext

import (
	"fmt"
	"strings"

	"github.com/manlikeabro/spotube/internal/matching"
)

// BriefExecutorLine summarizes an executor run for stdout logging.
func BriefExecutorLine(run ExecutorRun) string {
	src := strings.TrimSpace(run.Source.Title)
	if strings.TrimSpace(run.Source.Artist) != "" {
		src += " — " + strings.TrimSpace(run.Source.Artist)
	}
	selected := "none"
	if run.Selected != nil {
		selected = fmt.Sprintf("%q (%s)", strings.TrimSpace(run.Selected.Title), run.Selected.ID)
	}
	return fmt.Sprintf(
		"executor %s %s: %q → %s outcome=%s",
		run.Operation,
		run.Source.Platform,
		src,
		selected,
		run.Outcome,
	)
}

// AnalysisItemContext stores the full match decision on a sync item row.
type AnalysisItemContext struct {
	Version  int               `json:"version"`
	Decision matching.Decision `json:"decision"`
}
