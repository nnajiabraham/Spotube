package synccontext

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/manlikeabro/spotube/internal/matching"
)

func TestBriefExecutorLine(t *testing.T) {
	src := matching.Track{Title: "Umbrella", Artist: "Moses Bliss"}
	tests := []struct {
		name string
		run  ExecutorRun
		dest string
		want string
	}{
		{"added", ExecutorRun{Source: src, Outcome: "added", Selected: &matching.Track{Title: "Umbrella (Live)"}}, "youtube",
			`Added "Umbrella — Moses Bliss" to YouTube as "Umbrella (Live)"`},
		{"duplicate", ExecutorRun{Source: src, Outcome: "skipped_duplicate"}, "spotify",
			`"Umbrella — Moses Bliss" is already in the Spotify playlist`},
		{"no match", ExecutorRun{Source: src, Outcome: "no_match", Error: "no confident match on youtube (best score 0.65)"}, "youtube",
			`Couldn't find "Umbrella — Moses Bliss" on YouTube: no confident match on youtube (best score 0.65)`},
		{"renamed", ExecutorRun{Operation: "rename", Source: matching.Track{Title: "Road Trip"}, Outcome: "renamed"}, "spotify",
			`Renamed the Spotify playlist to "Road Trip"`},
		{"error", ExecutorRun{Operation: "add", Source: src, Outcome: "error", Error: "quota exceeded"}, "youtube",
			`Failed to add "Umbrella — Moses Bliss" on YouTube: quota exceeded`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, BriefExecutorLine(tt.run, tt.dest))
		})
	}
}
