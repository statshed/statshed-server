package store

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/statshed/statshed-server/internal/config"
)

// ConfigValues are the three global config settings (GET /api/config).
type ConfigValues struct {
	ProgressTimeoutMinutes int
	StalenessTimeoutHours  int
	ExpirationTimeoutHours int
}

// Config returns the global config values from the config table, falling back to defaults.
func (s *Store) Config(ctx context.Context) (ConfigValues, error) {
	progress, err := configInt(ctx, s.read, "progress_timeout_minutes", config.DefaultProgressTimeoutMinutes)
	if err != nil {
		return ConfigValues{}, err
	}
	staleness, err := configInt(ctx, s.read, "staleness_timeout_hours", config.DefaultStalenessTimeoutHours)
	if err != nil {
		return ConfigValues{}, err
	}
	expiration, err := configInt(ctx, s.read, "expiration_timeout_hours", config.DefaultExpirationTimeoutHours)
	if err != nil {
		return ConfigValues{}, err
	}
	return ConfigValues{
		ProgressTimeoutMinutes: progress,
		StalenessTimeoutHours:  staleness,
		ExpirationTimeoutHours: expiration,
	}, nil
}

// ConfigValue returns a single integer config value (key) or def when absent.
func (s *Store) ConfigValue(ctx context.Context, key string, def int) (int, error) {
	return configInt(ctx, s.read, key, def)
}

// SetConfigValue upserts an integer config value (JSON-encoded, matching set_config_value).
func (s *Store) SetConfigValue(ctx context.Context, key string, value int) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = s.write.ExecContext(ctx,
		"INSERT INTO config (key, value) VALUES (?, ?) "+
			"ON CONFLICT(key) DO UPDATE SET value = excluded.value",
		key, string(raw))
	return err
}

// CascadeGlobalExpiration refreshes expires_at = datetime(updated_at, '+N hours') for jobs
// in groups WITHOUT an expiration override, mirroring the Python global-config cascade
// (whole-second via SQLite datetime(), D13). hours is int-validated by the caller.
func (s *Store) CascadeGlobalExpiration(ctx context.Context, hours int) error {
	_, err := s.write.ExecContext(ctx,
		"UPDATE jobs SET expires_at = datetime(updated_at, ?) "+
			"WHERE group_id IN (SELECT id FROM groups WHERE expiration_timeout_hours IS NULL)",
		fmt.Sprintf("+%d hours", hours))
	return err
}
