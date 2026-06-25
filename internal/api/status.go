package api

import (
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/statshed/statshed-server/internal/config"
	"github.com/statshed/statshed-server/internal/realtime"
	"github.com/statshed/statshed-server/internal/store"
)

// namedPattern matches a normalized group/job name (behavioral-map §2).
var namePattern = regexp.MustCompile(`^[a-z0-9._-]+$`)

// sortedStatusList is the valid statuses sorted, for the "must be one of" message
// (matching Python's sorted(VALID_STATUSES)).
var sortedStatusList = func() string {
	s := append([]string(nil), store.ValidStatuses...)
	sort.Strings(s)
	return strings.Join(s, ", ")
}()

const logDisabledWarning = "Log uploads are disabled; log file was ignored"

func (h *handlers) postStatus(w http.ResponseWriter, r *http.Request) {
	data, logInput, warning, ok := h.parseStatusRequest(w, r)
	if !ok {
		return
	}

	groupName, errMsg := validateName(data["group"], "group", config.MaxGroupNameLength)
	if errMsg != "" {
		writeFieldError(w, http.StatusBadRequest, slugValidation, errMsg, "group")
		return
	}
	jobName, errMsg := validateName(data["job"], "job", config.MaxJobNameLength)
	if errMsg != "" {
		writeFieldError(w, http.StatusBadRequest, slugValidation, errMsg, "job")
		return
	}
	status, errMsg := validateStatus(data["status"])
	if errMsg != "" {
		writeFieldError(w, http.StatusBadRequest, slugValidation, errMsg, "status")
		return
	}
	message, errMsg := validateMessage(data["message"])
	if errMsg != "" {
		writeFieldError(w, http.StatusBadRequest, slugValidation, errMsg, "message")
		return
	}

	result, err := h.store.UpsertJob(r.Context(), store.UpsertParams{
		GroupName: groupName,
		JobName:   jobName,
		Status:    status,
		Message:   message,
		Log:       logInput,
	}, time.Now().UTC())
	if err != nil {
		slog.Error("upsert job", "err", err)
		writeError(w, http.StatusInternalServerError, slugInternal,
			"An internal server error occurred")
		return
	}

	// A brand-new group emits group_created first (carrying the full group), then every
	// status report emits status_update with the previous status (null on create).
	if result.GroupCreated {
		if group, found, gerr := h.store.GroupByName(r.Context(), result.Job.GroupName); gerr == nil && found {
			realtime.Publish(h.hub, "group_created", map[string]any{
				"schema_version": 1,
				"group":          group.APIMap(),
			})
		}
	}
	realtime.Publish(h.hub, "status_update", map[string]any{
		"schema_version":  1,
		"job":             result.Job.APIMap(),
		"group_id":        result.Job.GroupID,
		"group_name":      result.Job.GroupName,
		"previous_status": result.PreviousStatus,
	})

	resp := map[string]any{"success": true, "job": result.Job.APIMap()}
	if warning != "" {
		resp["warning"] = warning
	}
	writeJSON(w, http.StatusCreated, resp)
}

// parseStatusRequest extracts the fields (+ optional log) from a JSON or multipart body.
// It returns ok=false after having written an error response.
func (h *handlers) parseStatusRequest(w http.ResponseWriter, r *http.Request) (data map[string]any, logInput *store.LogInput, warning string, ok bool) {
	if strings.Contains(r.Header.Get("Content-Type"), "multipart/form-data") {
		return h.parseMultipart(w, r)
	}

	m, ok := decodeJSONObject(w, r)
	if !ok {
		return nil, nil, "", false
	}
	return m, nil, "", true
}

func (h *handlers) parseMultipart(w http.ResponseWriter, r *http.Request) (map[string]any, *store.LogInput, string, bool) {
	if err := r.ParseMultipartForm(config.MaxContentLength); err != nil {
		if writeIfTooLarge(w, err) {
			return nil, nil, "", false
		}
		writeError(w, http.StatusBadRequest, slugBadRequest, "Invalid multipart form")
		return nil, nil, "", false
	}
	data := map[string]any{
		"group":  r.FormValue("group"),
		"job":    r.FormValue("job"),
		"status": r.FormValue("status"),
	}
	// Record message iff the field is PRESENT (even when empty), so the multipart path matches
	// the JSON path and Python's request.form.get: present-empty -> "" (stored as ""), absent ->
	// NULL (I12). r.FormValue could not distinguish present-empty from absent.
	if vals, ok := r.MultipartForm.Value["message"]; ok && len(vals) > 0 {
		data["message"] = vals[0]
	}

	file, _, err := r.FormFile("log")
	if err != nil {
		return data, nil, "", true // no log part
	}
	defer func() { _ = file.Close() }()

	if !h.cfg.LogUploadEnabled {
		return data, nil, logDisabledWarning, true
	}
	content, lineCount, truncated, perr := processLog(file, h.cfg.MaxLogLines)
	if perr != nil {
		writeFieldError(w, http.StatusBadRequest, slugValidation,
			"Failed to read log file: "+perr.Error(), "log")
		return nil, nil, "", false
	}
	return data, &store.LogInput{Content: content, LineCount: lineCount, Truncated: truncated}, "", true
}

// validateName validates + normalizes a group/job name, returning (normalized, errMsg).
func validateName(value any, field string, maxLen int) (string, string) {
	s, isStr := value.(string)
	if !isStr {
		if value == nil {
			return "", field + " is required"
		}
		return "", field + " must be a string"
	}
	if s == "" {
		return "", field + " is required"
	}
	normalized := strings.ToLower(strings.TrimSpace(s))
	if utf8.RuneCountInString(normalized) > maxLen {
		return "", fmt.Sprintf("%s exceeds maximum length of %d characters", field, maxLen)
	}
	if !namePattern.MatchString(normalized) {
		return "", field + " contains invalid characters. " +
			"Only alphanumeric, dash, underscore, and dot are allowed."
	}
	return normalized, ""
}

func validateStatus(value any) (string, string) {
	if value != nil {
		if _, isStr := value.(string); !isStr {
			return "", "status must be a string"
		}
	}
	s, _ := value.(string)
	status := strings.ToLower(strings.TrimSpace(s))
	if status == "" {
		return "", "status is required"
	}
	if !store.IsValidStatus(status) {
		return "", "status must be one of: " + sortedStatusList
	}
	return status, ""
}

func validateMessage(value any) (*string, string) {
	if value == nil {
		return nil, ""
	}
	s, isStr := value.(string)
	if !isStr {
		return nil, "message must be a string"
	}
	if utf8.RuneCountInString(s) > config.MaxMessageLength {
		return nil, fmt.Sprintf("message exceeds maximum length of %d", config.MaxMessageLength)
	}
	return &s, ""
}

// processLog reads, decodes, and truncates an uploaded log to the last maxLines lines,
// mirroring Python's process_log_file (UTF-8 with latin-1 fallback; splitlines keepends).
func processLog(file multipart.File, maxLines int) (content string, lineCount int, truncated bool, err error) {
	b, err := io.ReadAll(file)
	if err != nil {
		return "", 0, false, err
	}
	text := decodeLog(b)
	lines := splitLinesKeepEnds(text)
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
		truncated = true
		text = strings.Join(lines, "")
	}
	return text, len(lines), truncated, nil
}

// decodeLog decodes log bytes as UTF-8, falling back to latin-1 (each byte -> the
// same-numbered code point) for non-UTF-8 input, matching Python's decode fallback.
func decodeLog(b []byte) string {
	if utf8.Valid(b) {
		return string(b)
	}
	runes := make([]rune, len(b))
	for i, c := range b {
		runes[i] = rune(c)
	}
	return string(runes)
}

// splitLinesKeepEnds splits s into lines keeping the terminators, matching Python's
// str.splitlines(keepends=True): it breaks on \n, \r and \r\n (as one terminator), and
// ALSO on \v \f \x1c \x1d \x1e \u0085 (NEL) \u2028 (LINE SEP) \u2029 (PARA SEP) (I13).
// \u0085/\u2028/\u2029 are multibyte in UTF-8, so we scan runes. A trailing terminator does
// NOT yield an extra empty final line.
func splitLinesKeepEnds(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		switch r {
		case '\r':
			end := i + size
			if end < len(s) && s[end] == '\n' { // \r\n is a single terminator
				end++
			}
			lines = append(lines, s[start:end])
			i, start = end, end
		case '\n', '\v', '\f', '\x1c', '\x1d', '\x1e', '\u0085', '\u2028', '\u2029':
			end := i + size
			lines = append(lines, s[start:end])
			i, start = end, end
		default:
			i += size
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
