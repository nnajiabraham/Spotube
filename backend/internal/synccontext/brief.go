package synccontext

import (
	"fmt"
	"strings"

	"github.com/manlikeabro/spotube/internal/matching"
)

// BriefExecutorLine is a one-line, human-readable summary of an executor run. dest is the
// service the item was applied to ("spotify" or "youtube").
func BriefExecutorLine(run ExecutorRun, dest string) string {
	src := strings.TrimSpace(run.Source.Title)
	if artist := strings.TrimSpace(run.Source.Artist); artist != "" {
		src += " — " + artist
	}
	service := serviceLabel(dest)

	switch run.Outcome {
	case "added":
		if run.Linked {
			return fmt.Sprintf("Added %q to %s (remembered match)", src, service)
		}
		if run.Selected != nil {
			return fmt.Sprintf("Added %q to %s as %q", src, service, strings.TrimSpace(run.Selected.Title))
		}
		return fmt.Sprintf("Added %q to %s", src, service)
	case "skipped_duplicate":
		return fmt.Sprintf("%q is already in the %s playlist", src, service)
	case "no_match":
		return fmt.Sprintf("Couldn't find %q on %s: %s", src, service, run.Error)
	case "not_available":
		return fmt.Sprintf("Skipped %q: marked as not on %s", src, service)
	case "renamed":
		return fmt.Sprintf("Renamed the %s playlist to %q", service, src)
	}

	verb := "add"
	if run.Operation == "rename" {
		verb = "rename to"
	}
	line := fmt.Sprintf("Failed to %s %q on %s", verb, src, service)
	if run.Error != "" {
		line += ": " + run.Error
	}
	return line
}

func serviceLabel(service string) string {
	switch service {
	case "youtube":
		return "YouTube"
	case "spotify":
		return "Spotify"
	}
	return service
}

// AnalysisItemContext stores the full match decision on a sync item row.
type AnalysisItemContext struct {
	Version  int               `json:"version"`
	Decision matching.Decision `json:"decision"`
}
