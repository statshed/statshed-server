package store

import "time"

// Job is a job row rendered for the API. The log_content blob is never loaded (HasLog is
// derived from log_content IS NOT NULL, spec §5.1).
type Job struct {
	ID           int
	GroupID      int
	GroupName    string
	Name         string
	Status       string
	Message      *string
	Acked        bool
	AckedAt      *time.Time
	ExpiresAt    *time.Time
	HasLog       bool
	LogLineCount *int
	LogTruncated bool
	LogUpdatedAt *time.Time
	UpdatedAt    time.Time
	CreatedAt    time.Time
}

// APIMap renders the job in the exact shape of the Python job.to_dict() (behavioral-map
// §2): timestamps as whole-second UTC strings, nullable fields as null (present, not
// absent).
func (j Job) APIMap() map[string]any {
	return map[string]any{
		"id":             j.ID,
		"group_id":       j.GroupID,
		"group_name":     j.GroupName,
		"name":           j.Name,
		"status":         j.Status,
		"message":        strOrNil(j.Message),
		"acked":          j.Acked,
		"acked_at":       apiTimeOrNil(j.AckedAt),
		"expires_at":     apiTimeOrNil(j.ExpiresAt),
		"has_log":        j.HasLog,
		"log_line_count": intOrNil(j.LogLineCount),
		"log_truncated":  j.LogTruncated,
		"log_updated_at": apiTimeOrNil(j.LogUpdatedAt),
		"updated_at":     formatAPI(j.UpdatedAt),
		"created_at":     formatAPI(j.CreatedAt),
	}
}

func apiTimeOrNil(t *time.Time) any {
	if t == nil {
		return nil
	}
	return formatAPI(*t)
}

func strOrNil(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}

func intOrNil(n *int) any {
	if n == nil {
		return nil
	}
	return *n
}
