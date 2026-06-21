//go:build contract

// Ported from contract/test_logs.py — job log upload retrieval + log config flags.
// Retrieval is default-profile; store-side limits run under max_log_lines (MAX_LOG_LINES=1500);
// the ignored-log warning runs under log_disabled (LOG_UPLOAD_ENABLED=false).
package contracttest

import (
	"fmt"
	"strings"
	"testing"
)

func logSubmit(t *testing.T, group, job, content string) map[string]any {
	t.Helper()
	status, body := postMultipart(t, "/api/status",
		map[string]string{"group": group, "job": job, "status": "success"},
		&multipartFile{Field: "log", Filename: "log.txt", Content: []byte(content)})
	mustStatus(t, status, 201)
	return body
}

// logLines mirrors _lines(n): n lines "line 0".."line {n-1}" joined by \n (no trailing newline).
func logLines(n int) string {
	parts := make([]string, n)
	for i := 0; i < n; i++ {
		parts[i] = fmt.Sprintf("line %d", i)
	}
	return strings.Join(parts, "\n")
}

// --- Retrieval (default profile) ---

func TestLogsGetFullContent(t *testing.T) {
	begin(t, "default")
	content := "line one\nline two\nline three"
	logSubmit(t, "builds", "test", content)
	status, body := getJSON(t, "/api/groups/builds/jobs/test/log")
	mustStatus(t, status, 200)
	mustEqStr(t, body, "log", content)
	mustEqNum(t, body, "line_count", 3)
	mustEqBool(t, body, "truncated", false)
}

func TestLogsGetWithTailParam(t *testing.T) {
	begin(t, "default")
	logSubmit(t, "builds", "test", logLines(100))
	status, body := getJSON(t, "/api/groups/builds/jobs/test/log?tail=5")
	mustStatus(t, status, 200)
	mustEqNum(t, body, "line_count", 5)
	mustEqBool(t, body, "truncated", true)
	mustEqNum(t, body, "total_line_count", 100)
}

func TestLogsGetWithAllParam(t *testing.T) {
	begin(t, "default")
	logSubmit(t, "builds", "test", logLines(100))
	status, body := getJSON(t, "/api/groups/builds/jobs/test/log?all=true")
	mustStatus(t, status, 200)
	mustEqNum(t, body, "line_count", 100)
	mustEqBool(t, body, "truncated", false)
}

func TestLogsGetNotFoundGroup(t *testing.T) {
	begin(t, "default")
	status, body := getJSON(t, "/api/groups/nonexistent/jobs/test/log")
	mustStatus(t, status, 404)
	mustContains(t, strings.ToLower(gstr(t, body, "message")), "not found")
}

func TestLogsGetNotFoundJob(t *testing.T) {
	begin(t, "default")
	logSubmit(t, "builds", "exists", "x")
	status, body := getJSON(t, "/api/groups/builds/jobs/nonexistent/log")
	mustStatus(t, status, 404)
	mustContains(t, strings.ToLower(gstr(t, body, "message")), "not found")
}

func TestLogsGetNoLogAvailable(t *testing.T) {
	begin(t, "default")
	postJSON(t, "/api/status", map[string]any{"group": "g", "job": "nolog", "status": "success"})
	status, body := getJSON(t, "/api/groups/g/jobs/nolog/log")
	mustStatus(t, status, 404)
	mustContains(t, strings.ToLower(gstr(t, body, "message")), "no log")
}

// --- Store-side limits (max_log_lines profile, MAX_LOG_LINES=1500) ---

func TestLogsGetDefaultTail1000(t *testing.T) {
	begin(t, "max_log_lines")
	logSubmit(t, "builds", "big", logLines(1500))
	status, body := getJSON(t, "/api/groups/builds/jobs/big/log")
	mustStatus(t, status, 200)
	mustEqNum(t, body, "line_count", 1000)
	mustEqBool(t, body, "truncated", true)
	mustEqNum(t, body, "total_line_count", 1500)
}

func TestLogsTruncationToMaxLines(t *testing.T) {
	begin(t, "max_log_lines")
	body := logSubmit(t, "builds", "huge", logLines(1600))
	job := gmap(t, body, "job")
	mustEqBool(t, job, "has_log", true)
	mustEqNum(t, job, "log_line_count", 1500)
	mustEqBool(t, job, "log_truncated", true)
}

func TestLogsWithinMaxLinesNotTruncated(t *testing.T) {
	begin(t, "max_log_lines")
	body := logSubmit(t, "builds", "small", logLines(50))
	job := gmap(t, body, "job")
	mustEqNum(t, job, "log_line_count", 50)
	mustEqBool(t, job, "log_truncated", false)
}

// --- Log upload disabled (log_disabled profile) ---

func TestLogsUploadDisabled(t *testing.T) {
	begin(t, "log_disabled")
	status, body := postMultipart(t, "/api/status",
		map[string]string{"group": "g", "job": "j", "status": "success"},
		&multipartFile{Field: "log", Filename: "log.txt", Content: []byte("a\nb\nc")})
	mustStatus(t, status, 201)
	job := gmap(t, body, "job")
	mustEqBool(t, job, "has_log", false)
	// The conditional `warning` key is present ONLY because a log was ignored.
	mustHave(t, body, "warning")
	mustContains(t, strings.ToLower(gstr(t, body, "warning")), "disabled")
}
