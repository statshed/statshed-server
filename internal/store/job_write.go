package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/statshed/statshed-server/internal/config"
)

// LogInput is processed log content to store on a job.
type LogInput struct {
	Content   string
	LineCount int
	Truncated bool
}

// UpsertParams is the input for creating or updating a job via POST /api/status.
type UpsertParams struct {
	GroupName string
	JobName   string
	Status    string
	Message   *string   // nil -> NULL
	Log       *LogInput // nil -> log left untouched on update, NULL on insert
}

// UpsertResult reports the upserted job plus what changed (for event emission).
type UpsertResult struct {
	Job            Job
	GroupCreated   bool
	PreviousStatus *string // nil on create
}

// scanCols is the column list (with has_log derived, log_content blob NOT loaded) used to
// read a job back in the API shape.
const scanCols = "id, group_id, name, status, message, acked, acked_at, expires_at, " +
	"(log_content IS NOT NULL), log_line_count, log_truncated, log_updated_at, " +
	"updated_at, created_at"

// UpsertJob creates or updates the (group, job) pair atomically and returns the rendered
// job. Writes go through the serialized write handle, so concurrent POSTs are processed
// one at a time — no IntegrityError-retry dance is needed (D7). expires_at is computed on
// the insert/update path as now + effective expiration (group override else global, else
// the default). Recovery to success/progress clears the ack; a log is replaced only when
// one is provided.
func (s *Store) UpsertJob(ctx context.Context, p UpsertParams, now time.Time) (UpsertResult, error) {
	tx, err := s.write.BeginTx(ctx, nil)
	if err != nil {
		return UpsertResult{}, err
	}
	defer func() { _ = tx.Rollback() }() // no-op after a successful commit

	nowStored := formatStored(now)

	// Get or create the group.
	res, err := tx.ExecContext(ctx,
		"INSERT INTO groups (name, created_at) VALUES (?, ?) ON CONFLICT(name) DO NOTHING",
		p.GroupName, nowStored)
	if err != nil {
		return UpsertResult{}, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return UpsertResult{}, err
	}
	groupCreated := affected == 1

	var groupID int
	var groupExpHours sql.NullInt64
	if err := tx.QueryRowContext(ctx,
		"SELECT id, expiration_timeout_hours FROM groups WHERE name = ?", p.GroupName,
	).Scan(&groupID, &groupExpHours); err != nil {
		return UpsertResult{}, err
	}

	// Effective expiration: group override, else the global config value, else the default.
	var expHours int
	if groupExpHours.Valid {
		expHours = int(groupExpHours.Int64)
	} else if expHours, err = configInt(ctx, tx, "expiration_timeout_hours", config.DefaultExpirationTimeoutHours); err != nil {
		return UpsertResult{}, err
	}
	expiresStored := formatStored(expiresInsert(now, expHours))

	var jobID int
	var prevStatus string
	err = tx.QueryRowContext(ctx,
		"SELECT id, status FROM jobs WHERE group_id = ? AND name = ?", groupID, p.JobName,
	).Scan(&jobID, &prevStatus)

	var previousStatus *string
	switch {
	case errors.Is(err, sql.ErrNoRows):
		if err := insertJobRow(ctx, tx, groupID, p, nowStored, expiresStored); err != nil {
			return UpsertResult{}, err
		}
	case err != nil:
		return UpsertResult{}, err
	default:
		previousStatus = &prevStatus
		if err := updateJobRow(ctx, tx, jobID, p, nowStored, expiresStored); err != nil {
			return UpsertResult{}, err
		}
	}

	job, err := scanJob(tx.QueryRowContext(ctx,
		"SELECT "+scanCols+" FROM jobs WHERE group_id = ? AND name = ?", groupID, p.JobName,
	), p.GroupName)
	if err != nil {
		return UpsertResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return UpsertResult{}, err
	}
	return UpsertResult{Job: job, GroupCreated: groupCreated, PreviousStatus: previousStatus}, nil
}

func insertJobRow(ctx context.Context, tx *sql.Tx, groupID int, p UpsertParams, nowStored, expiresStored string) error {
	var logContent, logUpdatedAt *string
	var logLineCount *int
	logTruncated := 0
	if p.Log != nil {
		logContent = &p.Log.Content
		lc := p.Log.LineCount
		logLineCount = &lc
		logTruncated = boolInt(p.Log.Truncated)
		logUpdatedAt = &nowStored
	}
	_, err := tx.ExecContext(ctx,
		"INSERT INTO jobs (group_id, name, status, message, acked, acked_at, expires_at, "+
			"log_content, log_line_count, log_truncated, log_updated_at, updated_at, created_at) "+
			"VALUES (?, ?, ?, ?, 0, NULL, ?, ?, ?, ?, ?, ?, ?)",
		groupID, p.JobName, p.Status, p.Message, expiresStored,
		logContent, logLineCount, logTruncated, logUpdatedAt, nowStored, nowStored)
	return err
}

func updateJobRow(ctx context.Context, tx *sql.Tx, jobID int, p UpsertParams, nowStored, expiresStored string) error {
	sets := []string{"status = ?", "message = ?", "updated_at = ?", "expires_at = ?"}
	args := []any{p.Status, p.Message, nowStored, expiresStored}
	// Recovery to a healthy state clears the ack so a future error needs a fresh ack.
	if p.Status == "success" || p.Status == "progress" {
		sets = append(sets, "acked = 0", "acked_at = NULL")
	}
	// A log replaces the previous one only when one was provided; otherwise it's preserved.
	if p.Log != nil {
		sets = append(sets, "log_content = ?", "log_line_count = ?", "log_truncated = ?", "log_updated_at = ?")
		args = append(args, p.Log.Content, p.Log.LineCount, boolInt(p.Log.Truncated), nowStored)
	}
	args = append(args, jobID)
	_, err := tx.ExecContext(ctx, "UPDATE jobs SET "+strings.Join(sets, ", ")+" WHERE id = ?", args...)
	return err
}

// scanJob reads a job row (in scanCols order) into the API-shaped Job.
func scanJob(row interface{ Scan(...any) error }, groupName string) (Job, error) {
	var j Job
	j.GroupName = groupName
	var message, ackedAt, expiresAt, logUpdatedAt sql.NullString
	var logLineCount sql.NullInt64
	var acked, hasLog, logTruncated int
	var updatedAt, createdAt string

	if err := row.Scan(&j.ID, &j.GroupID, &j.Name, &j.Status, &message, &acked,
		&ackedAt, &expiresAt, &hasLog, &logLineCount, &logTruncated, &logUpdatedAt,
		&updatedAt, &createdAt); err != nil {
		return Job{}, err
	}

	j.Message = nullStrPtr(message)
	j.Acked = acked != 0
	j.HasLog = hasLog != 0
	j.LogLineCount = nullIntPtr(logLineCount)
	j.LogTruncated = logTruncated != 0

	var err error
	if j.AckedAt, err = nullTimePtr(ackedAt); err != nil {
		return Job{}, err
	}
	if j.ExpiresAt, err = nullTimePtr(expiresAt); err != nil {
		return Job{}, err
	}
	if j.LogUpdatedAt, err = nullTimePtr(logUpdatedAt); err != nil {
		return Job{}, err
	}
	if j.UpdatedAt, err = parseStored(updatedAt); err != nil {
		return Job{}, err
	}
	if j.CreatedAt, err = parseStored(createdAt); err != nil {
		return Job{}, err
	}
	return j, nil
}

// configInt reads an integer config value (JSON-encoded in the config table), falling back
// to def when the key is absent.
func configInt(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, key string, def int) (int, error) {
	var raw string
	err := q.QueryRowContext(ctx, "SELECT value FROM config WHERE key = ?", key).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return def, nil
	}
	if err != nil {
		return 0, err
	}
	var n int
	if err := json.Unmarshal([]byte(raw), &n); err != nil {
		return 0, err
	}
	return n, nil
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func nullStrPtr(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	v := ns.String
	return &v
}

func nullIntPtr(ni sql.NullInt64) *int {
	if !ni.Valid {
		return nil
	}
	v := int(ni.Int64)
	return &v
}

func nullTimePtr(ns sql.NullString) (*time.Time, error) {
	if !ns.Valid {
		return nil, nil
	}
	t, err := parseStored(ns.String)
	if err != nil {
		return nil, err
	}
	return &t, nil
}
