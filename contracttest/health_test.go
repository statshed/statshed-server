//go:build contract

// Ported from contract/test_health.py — GET /api/health behavior + the CLI health shape.
package contracttest

import "testing"

func TestHealthEmpty(t *testing.T) {
	begin(t, "default")
	status, body := getJSON(t, "/api/health")
	mustStatus(t, status, 200)
	mustEqStr(t, body, "status", "empty")
	mustEqNum(t, body, "total_jobs", 0)
	mustEqNum(t, body, "healthy", 0)
	mustEqNum(t, body, "unhealthy", 0)
	mustEqNum(t, body, "in_progress", 0)
}

func TestHealthHealthy(t *testing.T) {
	begin(t, "default")
	status, _ := postJSON(t, "/api/status", map[string]any{"group": "test", "job": "job1", "status": "success"})
	mustStatus(t, status, 201)

	status, body := getJSON(t, "/api/health")
	mustStatus(t, status, 200)
	mustEqStr(t, body, "status", "healthy")
	mustEqNum(t, body, "total_jobs", 1)
	mustEqNum(t, body, "healthy", 1)
}

func TestHealthUnhealthyWithError(t *testing.T) {
	begin(t, "default")
	postJSON(t, "/api/status", map[string]any{"group": "test", "job": "job1", "status": "error"})

	_, body := getJSON(t, "/api/health")
	mustEqStr(t, body, "status", "unhealthy")
	mustEqNum(t, body, "unhealthy", 1)
}

func TestHealthInProgress(t *testing.T) {
	begin(t, "default")
	postJSON(t, "/api/status", map[string]any{"group": "test", "job": "job1", "status": "progress"})

	_, body := getJSON(t, "/api/health")
	mustEqStr(t, body, "status", "in_progress")
	mustEqNum(t, body, "in_progress", 1)
}

func TestHealthUnhealthyTakesPrecedence(t *testing.T) {
	begin(t, "default")
	postJSON(t, "/api/status", map[string]any{"group": "test", "job": "job1", "status": "progress"})
	postJSON(t, "/api/status", map[string]any{"group": "test", "job": "job2", "status": "error"})

	_, body := getJSON(t, "/api/health")
	mustEqStr(t, body, "status", "unhealthy")
}

// Ported from contract/test_health.py::TestCliIntegration — the health shape the CLI depends on.
func TestCliHealthCheck(t *testing.T) {
	begin(t, "default")
	status, body := getJSON(t, "/api/health")
	mustStatus(t, status, 200)

	s := gstr(t, body, "status")
	switch s {
	case "healthy", "unhealthy", "in_progress", "empty":
	default:
		t.Errorf("status = %q, want one of healthy/unhealthy/in_progress/empty", s)
	}
	mustHave(t, body, "total_jobs")
	mustHave(t, body, "by_status")
}
