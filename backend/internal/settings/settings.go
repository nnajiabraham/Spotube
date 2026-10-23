// Package settings holds the app's runtime settings: a registry of known keys, each with a
// default, an optional environment variable and a description.
//
// Resolution:
//   - An environment variable set at startup wins at startup. The Settings page can still
//     change it; that change lives in memory until the next restart, when the env value
//     returns ("runtime").
//   - Otherwise the value is stored in SQLite (app_settings) and survives restarts
//     ("database"), or falls back to its default ("default").
package settings

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Kind is a setting's value type.
type Kind string

const (
	KindBool Kind = "bool"
	KindInt  Kind = "int"
)

// Keys of the known settings.
const (
	SyncWorkersEnabled        = "sync_workers_enabled"
	AutoApplyEnabled          = "auto_apply_enabled"
	BackupsEnabled            = "backups_enabled"
	YouTubeDailyQuota         = "youtube_daily_quota"
	YouTubeInteractiveReserve = "youtube_interactive_reserve"
	DrainItemsPerTick         = "drain_items_per_tick"
	SyncNewMappingsNow        = "sync_new_mappings_now"
)

// Def describes one setting.
type Def struct {
	Key         string `json:"key"`
	Env         string `json:"env,omitempty"`
	Kind        Kind   `json:"kind"`
	Default     string `json:"default"`
	Min         int    `json:"min,omitempty"`
	Max         int    `json:"max,omitempty"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Group       string `json:"group"`
}

// Registry lists every setting, in the order the Settings page shows them.
var Registry = []Def{
	{Key: SyncWorkersEnabled, Env: "SYNC_WORKERS_ENABLED", Kind: KindBool, Default: "false", Group: "sync",
		Label:       "Background sync",
		Description: "Run each mapping on its schedule. Off: mappings only sync when you press Sync now."},
	{Key: AutoApplyEnabled, Env: "SYNC_EXECUTOR_AUTO_ENABLED", Kind: KindBool, Default: "true", Group: "sync",
		Label:       "Apply changes automatically",
		Description: "Copy queued tracks without waiting for you. Off: tracks wait on the Queue page until you apply them."},
	{Key: SyncNewMappingsNow, Kind: KindBool, Default: "true", Group: "sync",
		Label:       "Sync new mappings straight away",
		Description: "Default for the \"Start syncing now\" box when pairing playlists."},
	{Key: DrainItemsPerTick, Kind: KindInt, Default: "100", Min: 1, Max: 1000, Group: "sync",
		Label:       "Tracks applied per minute",
		Description: "Most queued tracks the background sync applies in one pass (it runs every minute), while quota allows."},
	{Key: YouTubeDailyQuota, Env: "YOUTUBE_DAILY_QUOTA", Kind: KindInt, Default: "10000", Min: 100, Max: 10_000_000, Group: "quota",
		Label:       "YouTube daily quota",
		Description: "Units your Google Cloud project may use per day (10,000 unless Google raised it). Resets at midnight Pacific."},
	{Key: YouTubeInteractiveReserve, Kind: KindInt, Default: "500", Min: 0, Max: 10_000_000, Group: "quota",
		Label:       "Kept for you",
		Description: "Units background sync never touches, so searches and actions you take still work late in the day (a search costs 101)."},
	{Key: BackupsEnabled, Env: "BACKUPS_ENABLED", Kind: KindBool, Default: "true", Group: "backups",
		Label:       "Scheduled backups",
		Description: "Snapshot backed-up playlists on their own schedule. Backups also update for free whenever a sync reads the playlist."},
}

// Source says where a setting's current value comes from.
type Source string

const (
	SourceDefault  Source = "default"
	SourceEnv      Source = "env"
	SourceDatabase Source = "database"
	SourceRuntime  Source = "runtime"
)

// View is a setting with its current value, for the API.
type View struct {
	Def
	Value     string `json:"value"`
	Source    Source `json:"source"`
	EnvPinned bool   `json:"env_pinned"`
	EnvValue  string `json:"env_value,omitempty"`
}

// ErrUnknown is returned for a key that is not in the registry.
var ErrUnknown = errors.New("unknown setting")

// Store resolves settings. It is safe for concurrent use.
type Store struct {
	db     *sql.DB
	mu     sync.RWMutex
	values map[string]string
	source map[string]Source
	env    map[string]string // env values read at startup
}

// Load reads env vars (via lookup, os.LookupEnv when nil) and the app_settings table.
func Load(db *sql.DB, lookup func(string) (string, bool)) (*Store, error) {
	if lookup == nil {
		lookup = os.LookupEnv
	}
	s := &Store{db: db, values: map[string]string{}, source: map[string]Source{}, env: map[string]string{}}
	stored := map[string]string{}
	if db != nil {
		rows, err := db.Query(`SELECT key, value FROM app_settings`)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var k, v string
			if err := rows.Scan(&k, &v); err != nil {
				rows.Close()
				return nil, err
			}
			stored[k] = v
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	for _, d := range Registry {
		s.values[d.Key], s.source[d.Key] = d.Default, SourceDefault
		if d.Env != "" {
			if raw, ok := lookup(d.Env); ok && strings.TrimSpace(raw) != "" {
				v, err := normalize(d, raw)
				if err != nil {
					return nil, fmt.Errorf("invalid %s: %w", d.Env, err)
				}
				s.env[d.Key] = v
				s.values[d.Key], s.source[d.Key] = v, SourceEnv
				continue
			}
		}
		if raw, ok := stored[d.Key]; ok {
			// A stored value that no longer validates (e.g. bounds changed) falls back to the default.
			if v, err := normalize(d, raw); err == nil {
				s.values[d.Key], s.source[d.Key] = v, SourceDatabase
			}
		}
	}
	return s, nil
}

// Static returns a store with fixed values and no database, for tests.
func Static(values map[string]string) *Store {
	s, _ := Load(nil, func(string) (string, bool) { return "", false })
	for k, v := range values {
		s.values[k] = v
	}
	return s
}

func lookupDef(key string) (Def, bool) {
	for _, d := range Registry {
		if d.Key == key {
			return d, true
		}
	}
	return Def{}, false
}

func normalize(d Def, raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	switch d.Kind {
	case KindBool:
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return "", errors.New("must be true or false")
		}
		return strconv.FormatBool(b), nil
	case KindInt:
		n, err := strconv.Atoi(raw)
		if err != nil {
			return "", errors.New("must be a whole number")
		}
		if n < d.Min || (d.Max > 0 && n > d.Max) {
			return "", fmt.Errorf("must be between %d and %d", d.Min, d.Max)
		}
		return strconv.Itoa(n), nil
	}
	return raw, nil
}

// Bool returns a boolean setting (false for unknown keys).
func (s *Store) Bool(key string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, _ := strconv.ParseBool(s.values[key])
	return b
}

// Int returns an integer setting (0 for unknown keys).
func (s *Store) Int(key string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n, _ := strconv.Atoi(s.values[key])
	return n
}

// Validate checks values without applying any.
func (s *Store) Validate(values map[string]string) error {
	for key, raw := range values {
		d, ok := lookupDef(key)
		if !ok {
			return fmt.Errorf("%w: %s", ErrUnknown, key)
		}
		if _, err := normalize(d, raw); err != nil {
			return fmt.Errorf("%s %w", d.Label, err)
		}
	}
	return nil
}

// Set validates and applies a value. Env-pinned settings change in memory only; others are
// stored.
func (s *Store) Set(key, raw string) (View, error) {
	d, ok := lookupDef(key)
	if !ok {
		return View{}, fmt.Errorf("%w: %s", ErrUnknown, key)
	}
	v, err := normalize(d, raw)
	if err != nil {
		return View{}, fmt.Errorf("%s %w", d.Label, err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, pinned := s.env[key]; pinned {
		s.values[key] = v
		s.source[key] = SourceRuntime
		if v == s.env[key] {
			s.source[key] = SourceEnv
		}
		return s.viewLocked(d), nil
	}
	if s.db != nil {
		if _, err := s.db.Exec(
			`INSERT INTO app_settings (key, value, updated) VALUES (?, ?, ?)
			 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated = excluded.updated`,
			key, v, time.Now().Unix()); err != nil {
			return View{}, err
		}
	}
	s.values[key] = v
	s.source[key] = SourceDatabase
	return s.viewLocked(d), nil
}

// List returns every setting with its value and source.
func (s *Store) List() []View {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]View, 0, len(Registry))
	for _, d := range Registry {
		out = append(out, s.viewLocked(d))
	}
	return out
}

func (s *Store) viewLocked(d Def) View {
	envValue, pinned := s.env[d.Key]
	return View{Def: d, Value: s.values[d.Key], Source: s.source[d.Key], EnvPinned: pinned, EnvValue: envValue}
}
