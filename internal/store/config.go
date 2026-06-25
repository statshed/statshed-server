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

// cascadeGlobalExpirationSQL refreshes expires_at = datetime(updated_at, '+N hours') for jobs in
// groups WITHOUT an expiration override, mirroring the Python global-config cascade
// (whole-second via SQLite datetime(), D13). Shared by the standalone method and the in-tx path.
const cascadeGlobalExpirationSQL = "UPDATE jobs SET expires_at = datetime(updated_at, ?) " +
	"WHERE group_id IN (SELECT id FROM groups WHERE expiration_timeout_hours IS NULL)"

// CascadeGlobalExpiration runs the global-expiration cascade on the write handle. hours is
// int-validated by the caller.
func (s *Store) CascadeGlobalExpiration(ctx context.Context, hours int) error {
	_, err := s.write.ExecContext(ctx, cascadeGlobalExpirationSQL, fmt.Sprintf("+%d hours", hours))
	return err
}

// SetGlobalConfig writes the given global config values and, when expiration_timeout_hours
// actually changes, runs the expiration cascade — all in ONE write transaction (I7). The
// handler validates every field BEFORE calling this, so a rejected request writes nothing and a
// multi-field update is applied all-or-nothing (the pre-fix handler wrote each field in a loop,
// leaving a partial update when a later field failed validation). The cascade runs on the SAME
// tx, NOT via CascadeGlobalExpiration: s.write is SetMaxOpenConns(1), so a nested call would
// deadlock on the held connection.
func (s *Store) SetGlobalConfig(ctx context.Context, updates map[string]int) error {
	if len(updates) == 0 {
		return nil
	}
	tx, err := s.write.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }() // no-op after Commit

	// Detect a real expiration change (for the cascade) against the value already stored.
	cascadeExp, changed := updates["expiration_timeout_hours"]
	if changed {
		old, err := configInt(ctx, tx, "expiration_timeout_hours", config.DefaultExpirationTimeoutHours)
		if err != nil {
			return err
		}
		changed = cascadeExp != old
	}

	for key, value := range updates {
		raw, err := json.Marshal(value)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			"INSERT INTO config (key, value) VALUES (?, ?) "+
				"ON CONFLICT(key) DO UPDATE SET value = excluded.value",
			key, string(raw)); err != nil {
			return err
		}
	}

	if changed {
		if _, err := tx.ExecContext(ctx, cascadeGlobalExpirationSQL,
			fmt.Sprintf("+%d hours", cascadeExp)); err != nil {
			return err
		}
	}
	return tx.Commit()
}
