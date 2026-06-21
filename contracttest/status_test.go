//go:build contract

// Ported from contract/test_status.py — POST /api/status behavior (create/update,
// normalization, validation, multipart log upload, encoding handling).
package contracttest

import (
	"strings"
	"testing"
)

func TestSubmitStatusCreatesJob(t *testing.T) {
	begin(t, "default")
	status, body := postJSON(t, "/api/status", map[string]any{
		"group": "backups", "job": "daily-backup", "status": "success", "message": "Completed in 45s",
	})
	mustStatus(t, status, 201)
	mustEqBool(t, body, "success", true)
	job := gmap(t, body, "job")
	mustEqStr(t, job, "name", "daily-backup")
	mustEqStr(t, job, "status", "success")
	mustEqStr(t, job, "message", "Completed in 45s")
	mustEqStr(t, job, "group_name", "backups")
}

func TestSubmitStatusUpdatesExisting(t *testing.T) {
	begin(t, "default")
	postJSON(t, "/api/status", map[string]any{"group": "test", "job": "job1", "status": "progress"})

	status, body := postJSON(t, "/api/status", map[string]any{"group": "test", "job": "job1", "status": "success"})
	mustStatus(t, status, 201)
	job := gmap(t, body, "job")
	mustEqStr(t, job, "status", "success")
}

func TestSubmitStatusMissingGroup(t *testing.T) {
	begin(t, "default")
	status, body := postJSON(t, "/api/status", map[string]any{"job": "job1", "status": "success"})
	mustStatus(t, status, 400)
	mustEqStr(t, body, "error", "validation_error")
	mustEqStr(t, body, "field", "group")
}

func TestSubmitStatusMissingJob(t *testing.T) {
	begin(t, "default")
	status, body := postJSON(t, "/api/status", map[string]any{"group": "test", "status": "success"})
	mustStatus(t, status, 400)
	mustEqStr(t, body, "field", "job")
}

func TestSubmitStatusMissingStatus(t *testing.T) {
	begin(t, "default")
	status, body := postJSON(t, "/api/status", map[string]any{"group": "test", "job": "job1"})
	mustStatus(t, status, 400)
	mustEqStr(t, body, "field", "status")
}

func TestSubmitStatusInvalidStatus(t *testing.T) {
	begin(t, "default")
	status, body := postJSON(t, "/api/status", map[string]any{"group": "test", "job": "job1", "status": "invalid"})
	mustStatus(t, status, 400)
	mustContains(t, gstr(t, body, "message"), "status must be one of")
}

func TestSubmitStatusNormalizesNames(t *testing.T) {
	begin(t, "default")
	status, body := postJSON(t, "/api/status", map[string]any{"group": "MyGroup", "job": "MyJob", "status": "success"})
	mustStatus(t, status, 201)
	job := gmap(t, body, "job")
	mustEqStr(t, job, "group_name", "mygroup")
	mustEqStr(t, job, "name", "myjob")
}

func TestSubmitStatusInvalidGroupName(t *testing.T) {
	begin(t, "default")
	status, body := postJSON(t, "/api/status", map[string]any{"group": "my group!", "job": "job1", "status": "success"})
	mustStatus(t, status, 400)
	mustContains(t, gstr(t, body, "message"), "invalid characters")
}

func TestSubmitStatusMessageTooLong(t *testing.T) {
	begin(t, "default")
	status, body := postJSON(t, "/api/status", map[string]any{
		"group": "test", "job": "job1", "status": "success", "message": strings.Repeat("x", 5000),
	})
	mustStatus(t, status, 400)
	mustContains(t, gstr(t, body, "message"), "maximum length")
}

// Re-expresses test_integration.py::TestIntegrityErrorRetry::test_group_and_job_both_exist
// as an over-the-wire create-then-update workflow (B2).
func TestStatusGroupAndJobBothExist(t *testing.T) {
	begin(t, "default")
	status, created := postJSON(t, "/api/status", map[string]any{
		"group": "existing-group", "job": "existing-job", "status": "progress", "message": "Old message",
	})
	mustStatus(t, status, 201)
	jobID := gnum(t, gmap(t, created, "job"), "id")

	status, body := postJSON(t, "/api/status", map[string]any{
		"group": "existing-group", "job": "existing-job", "status": "success", "message": "New message",
	})
	mustStatus(t, status, 201)
	job := gmap(t, body, "job")
	mustEqNum(t, job, "id", jobID)
	mustEqStr(t, job, "status", "success")
	mustEqStr(t, job, "message", "New message")
}

func TestStatusCliSubmitWorkflow(t *testing.T) {
	begin(t, "default")
	status, body := postJSON(t, "/api/status", map[string]any{
		"group": "nightly-builds", "job": "unit-tests", "status": "progress", "message": "Running tests...",
	})
	mustStatus(t, status, 201)
	mustEqBool(t, body, "success", true)
	mustEqStr(t, gmap(t, body, "job"), "status", "progress")

	status, body = postJSON(t, "/api/status", map[string]any{
		"group": "nightly-builds", "job": "unit-tests", "status": "success", "message": "All 42 tests passed",
	})
	mustStatus(t, status, 201)
	job := gmap(t, body, "job")
	mustEqStr(t, job, "status", "success")
	mustEqStr(t, job, "message", "All 42 tests passed")
}

func TestSubmitStatusWithLogMultipart(t *testing.T) {
	begin(t, "default")
	status, body := postMultipart(t, "/api/status",
		map[string]string{"group": "builds", "job": "compile", "status": "success", "message": "Build completed"},
		&multipartFile{Field: "log", Filename: "build.log", Content: []byte("line1\nline2\nline3\n")})
	mustStatus(t, status, 201)
	mustEqBool(t, body, "success", true)
	job := gmap(t, body, "job")
	mustEqBool(t, job, "has_log", true)
	mustEqNum(t, job, "log_line_count", 3)
	mustEqBool(t, job, "log_truncated", false)
	if isNull(job, "log_updated_at") || !has(job, "log_updated_at") {
		t.Errorf("log_updated_at = %v, want non-null", job["log_updated_at"])
	}
}

func TestSubmitStatusWithoutLogJSON(t *testing.T) {
	begin(t, "default")
	status, body := postJSON(t, "/api/status", map[string]any{"group": "builds", "job": "test", "status": "success"})
	mustStatus(t, status, 201)
	job := gmap(t, body, "job")
	mustEqBool(t, job, "has_log", false)
	mustNull(t, job, "log_line_count")
}

func TestSubmitStatusMultipartWithoutLog(t *testing.T) {
	begin(t, "default")
	// Multipart form fields but NO file part -> the server takes the multipart path with no log.
	status, body := postMultipart(t, "/api/status",
		map[string]string{"group": "builds", "job": "deploy", "status": "success"}, nil)
	mustStatus(t, status, 201)
	mustEqBool(t, gmap(t, body, "job"), "has_log", false)
}

func TestLogReplacesPreviousOnUpdate(t *testing.T) {
	begin(t, "default")
	status, _ := postMultipart(t, "/api/status",
		map[string]string{"group": "builds", "job": "test", "status": "progress"},
		&multipartFile{Field: "log", Filename: "log.txt", Content: []byte("initial log\n")})
	mustStatus(t, status, 201)

	_, body := getJSON(t, "/api/groups/builds/jobs/test/log")
	mustContains(t, gstr(t, body, "log"), "initial log")

	status, body = postMultipart(t, "/api/status",
		map[string]string{"group": "builds", "job": "test", "status": "success"},
		&multipartFile{Field: "log", Filename: "log.txt", Content: []byte("new log line 1\nnew log line 2\n")})
	mustStatus(t, status, 201)
	mustEqNum(t, gmap(t, body, "job"), "log_line_count", 2)

	_, body = getJSON(t, "/api/groups/builds/jobs/test/log")
	log := gstr(t, body, "log")
	mustNotContains(t, log, "initial log")
	mustContains(t, log, "new log line 1")
}

func TestLogNotClearedWithoutNewLog(t *testing.T) {
	begin(t, "default")
	status, _ := postMultipart(t, "/api/status",
		map[string]string{"group": "builds", "job": "test", "status": "progress"},
		&multipartFile{Field: "log", Filename: "log.txt", Content: []byte("my log content\n")})
	mustStatus(t, status, 201)

	status, body := postJSON(t, "/api/status", map[string]any{"group": "builds", "job": "test", "status": "success"})
	mustStatus(t, status, 201)
	job := gmap(t, body, "job")
	mustEqBool(t, job, "has_log", true)
	mustEqNum(t, job, "log_line_count", 1)
}

func TestLogMetadataInJobResponse(t *testing.T) {
	begin(t, "default")
	_, body := postMultipart(t, "/api/status",
		map[string]string{"group": "builds", "job": "test", "status": "success"},
		&multipartFile{Field: "log", Filename: "log.txt", Content: []byte(strings.Repeat("line\n", 10))})
	job := gmap(t, body, "job")
	mustHave(t, job, "has_log")
	mustHave(t, job, "log_line_count")
	mustHave(t, job, "log_truncated")
	mustHave(t, job, "log_updated_at")
	mustEqNum(t, job, "log_line_count", 10)
}

func TestLogUTF8Content(t *testing.T) {
	begin(t, "default")
	logContent := "Hello 世界\nUnicode: émojis 🎉\n"
	status, body := postMultipart(t, "/api/status",
		map[string]string{"group": "builds", "job": "test", "status": "success"},
		&multipartFile{Field: "log", Filename: "log.txt", Content: []byte(logContent)})
	mustStatus(t, status, 201)
	mustEqBool(t, gmap(t, body, "job"), "has_log", true)

	_, body = getJSON(t, "/api/groups/builds/jobs/test/log")
	mustContains(t, gstr(t, body, "log"), logContent)
}

func TestLogLatin1Fallback(t *testing.T) {
	begin(t, "default")
	// Bytes that are valid latin-1 but not valid UTF-8.
	status, body := postMultipart(t, "/api/status",
		map[string]string{"group": "builds", "job": "test", "status": "success"},
		&multipartFile{Field: "log", Filename: "log.txt", Content: []byte("Line with latin-1: caf\xe9\n")})
	mustStatus(t, status, 201)
	mustEqBool(t, gmap(t, body, "job"), "has_log", true)
}
