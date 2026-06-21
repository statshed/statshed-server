//go:build contract

// Package contracttest is the black-box HTTP contract / regression suite for the StatShed
// server, ported from the Python contract/ harness (Task 8.5). It drives the REAL built
// binary over HTTP — never the handlers in-process — so it asserts the wire contract, not
// internal state.
//
// TestMain builds and boots cmd/statshed-server once per `go test` invocation, under the
// profile named by CONTRACT_PROFILE (default "default"), on a fresh temp SQLite DB with
// STATSHED_TEST_HOOKS=1 (the 60s scheduler off; the POST /api/admin/run-checks tick hook
// on). It mirrors runner.py. Per-test isolation is direct-SQLite truncation, mirroring
// conftest.reset_db; backdate/insertGroup mirror the conftest helpers.
//
// Build-tagged `contract` so it is excluded from the fast `go test ./...` unit run; the
// contract CI job runs `go test -tags contract ./contracttest` once per profile.
package contracttest

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

var (
	baseURL    string
	dbFile     string
	profile    string
	testDB     *sql.DB
	httpClient = &http.Client{Timeout: 15 * time.Second}
)

// profileEnv mirrors runner.py PROFILE_ENV (the SPA profiles are resolved in serverEnv).
var profileEnv = map[string]map[string]string{
	"default":       {},
	"log_disabled":  {"LOG_UPLOAD_ENABLED": "false"},
	"max_log_lines": {"MAX_LOG_LINES": "1500"},
	"max_page_size": {"MAX_JOBS_PAGE_SIZE": "2"},
	"with_spa":      {},
	"no_spa":        {},
}

func TestMain(m *testing.M) { os.Exit(runMain(m)) }

func runMain(m *testing.M) (code int) {
	profile = os.Getenv("CONTRACT_PROFILE")
	if profile == "" {
		profile = "default"
	}
	if _, ok := profileEnv[profile]; !ok {
		fmt.Fprintf(os.Stderr, "unknown CONTRACT_PROFILE %q\n", profile)
		return 2
	}

	tmp, err := os.MkdirTemp("", "contracttest-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "mkdtemp:", err)
		return 1
	}
	defer func() { _ = os.RemoveAll(tmp) }()

	root := repoRoot()
	bin := filepath.Join(tmp, "statshed-server")
	build := exec.Command("go", "build", "-o", bin, "./cmd/statshed-server")
	build.Dir = root
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "build server: %v\n%s", err, out)
		return 1
	}

	dbFile = filepath.Join(tmp, "statshed.db")
	port := freePort()
	baseURL = fmt.Sprintf("http://127.0.0.1:%d", port)

	logPath := filepath.Join(tmp, "server.log")
	logFile, _ := os.Create(logPath)
	defer func() { _ = logFile.Close() }()

	cmd := exec.Command(bin)
	cmd.Dir = root
	cmd.Env = serverEnv(profile, port, dbFile, tmp)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		fmt.Fprintln(os.Stderr, "start server:", err)
		return 1
	}
	defer terminate(cmd)

	if err := waitHealthy(baseURL, 40*time.Second, cmd); err != nil {
		if b, rerr := os.ReadFile(logPath); rerr == nil {
			fmt.Fprintf(os.Stderr, "server log:\n%s\n", b)
		}
		fmt.Fprintln(os.Stderr, "server not healthy:", err)
		return 1
	}

	testDB, err = sql.Open("sqlite", "file:"+dbFile+"?_pragma=busy_timeout(5000)")
	if err != nil {
		fmt.Fprintln(os.Stderr, "open test db:", err)
		return 1
	}
	defer func() { _ = testDB.Close() }()

	return m.Run()
}

// repoRoot is the parent of the contracttest package dir (go test runs with CWD = pkg dir).
func repoRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return filepath.Dir(wd)
}

func freePort() int {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	defer func() { _ = l.Close() }()
	return l.Addr().(*net.TCPAddr).Port
}

// serverEnv builds the server process environment for a profile (mirrors runner.build_env).
func serverEnv(prof string, port int, db, tmp string) []string {
	env := os.Environ()
	set := func(k, v string) { env = append(env, k+"="+v) }
	set("HOST", "127.0.0.1")
	set("PORT", strconv.Itoa(port))
	// Absolute db path -> the 4-slash sqlite URL form ("sqlite:///" + "/abs/path").
	set("DATABASE_URL", "sqlite:///"+db)
	set("STATSHED_TEST_HOOKS", "1")
	for k, v := range profileEnv[prof] {
		set(k, v)
	}
	switch prof {
	case "with_spa":
		set("STATIC_DIR", writeSyntheticSPA(filepath.Join(tmp, "spa-dist")))
	case "no_spa":
		set("STATIC_DISABLED", "1")
	}
	return env
}

// writeSyntheticSPA writes the minimal dist the with_spa profile serves (mirrors runner.py):
// index.html containing "StatShed", assets/app.js containing "console.log".
func writeSyntheticSPA(dist string) string {
	_ = os.MkdirAll(filepath.Join(dist, "assets"), 0o755)
	_ = os.WriteFile(filepath.Join(dist, "index.html"),
		[]byte("<!doctype html><html><head><title>StatShed</title></head>"+
			`<body><div id="root"></div></body></html>`), 0o644)
	_ = os.WriteFile(filepath.Join(dist, "assets", "app.js"), []byte("console.log('hi')\n"), 0o644)
	return dist
}

func waitHealthy(base string, timeout time.Duration, cmd *exec.Cmd) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
			return fmt.Errorf("server exited early")
		}
		resp, err := httpClient.Get(base + "/api/health")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == 200 {
				return nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("not healthy within %s", timeout)
}

func terminate(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	_ = cmd.Process.Signal(syscall.SIGTERM)
	done := make(chan struct{})
	go func() { _, _ = cmd.Process.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		_ = cmd.Process.Kill()
	}
}

// --- per-test lifecycle ---

// begin gates a test to its profile and resets the DB. A test for a profile other than the
// running one skips (mirrors `pytest -m <profile>`); otherwise the tables are truncated to a
// pristine state (mirrors the autouse reset_db fixture).
func begin(t *testing.T, prof string) {
	t.Helper()
	if profile != prof {
		t.Skipf("profile %q test skipped under profile %q", prof, profile)
	}
	resetDB(t)
}

// resetDB truncates jobs -> groups -> config (FK order; no sqlite_sequence — no AUTOINCREMENT).
func resetDB(t *testing.T) {
	t.Helper()
	for _, stmt := range []string{"DELETE FROM jobs", "DELETE FROM groups", "DELETE FROM config"} {
		if _, err := testDB.Exec(stmt); err != nil {
			t.Fatalf("reset db (%s): %v", stmt, err)
		}
	}
}

// backdate sets columns on the rows matching where, via direct SQL — used to age rows without
// sleeping (mirrors conftest.backdate). table/where are trusted test literals.
func backdate(t *testing.T, table, where string, cols map[string]any) {
	t.Helper()
	names := make([]string, 0, len(cols))
	args := make([]any, 0, len(cols))
	for name, v := range cols {
		names = append(names, name+" = ?")
		args = append(args, v)
	}
	q := fmt.Sprintf("UPDATE %s SET %s WHERE %s", table, strings.Join(names, ", "), where)
	if _, err := testDB.Exec(q, args...); err != nil {
		t.Fatalf("backdate: %v", err)
	}
}

// insertGroup inserts a bare group (no jobs) via direct SQL — a zero-job group cannot be made
// through the API (mirrors conftest.insert_group). created_at uses the stored text format.
func insertGroup(t *testing.T, name string) {
	t.Helper()
	now := time.Now().UTC().Format("2006-01-02 15:04:05.000000")
	if _, err := testDB.Exec(
		"INSERT INTO groups (name, staleness_enabled, created_at) VALUES (?, 0, ?)", name, now); err != nil {
		t.Fatalf("insert group: %v", err)
	}
}

// --- HTTP request helpers ---

// doJSON performs a request with an optional JSON body and decodes the JSON response object.
func doJSON(t *testing.T, method, path string, payload any) (int, map[string]any) {
	t.Helper()
	var body io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("marshal payload: %v", err)
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, baseURL+path, body)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return sendDecode(t, req)
}

func getJSON(t *testing.T, path string) (int, map[string]any) {
	t.Helper()
	return doJSON(t, http.MethodGet, path, nil)
}

func postJSON(t *testing.T, path string, payload any) (int, map[string]any) {
	t.Helper()
	return doJSON(t, http.MethodPost, path, payload)
}

func putJSON(t *testing.T, path string, payload any) (int, map[string]any) {
	t.Helper()
	return doJSON(t, http.MethodPut, path, payload)
}

// deleteJSON issues DELETE with no body (and decodes the JSON response).
func deleteJSON(t *testing.T, path string) (int, map[string]any) {
	t.Helper()
	return doJSON(t, http.MethodDelete, path, nil)
}

// deleteJSONBody issues DELETE with a JSON body (admin cleanup) and decodes the response.
func deleteJSONBody(t *testing.T, path string, payload any) (int, map[string]any) {
	t.Helper()
	return doJSON(t, http.MethodDelete, path, payload)
}

// postRawJSON posts an arbitrary raw body with an explicit content type (malformed-JSON /
// wrong-content-type error tests).
func postRawJSON(t *testing.T, path, contentType, raw string) (int, map[string]any) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, baseURL+path, strings.NewReader(raw))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return sendDecode(t, req)
}

// multipartFile is an optional file part for postMultipart. A zero Field means "no file".
type multipartFile struct {
	Field    string
	Filename string
	Content  []byte
}

// postMultipart sends a multipart/form-data body: every fields entry as a form field, plus an
// optional file part. With no file (file == nil) it still sends multipart (not urlencoded), so
// the server takes the multipart path with no log — mirroring the Python `files={...}` form.
func postMultipart(t *testing.T, path string, fields map[string]string, file *multipartFile) (int, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			t.Fatalf("write field: %v", err)
		}
	}
	if file != nil {
		fw, err := w.CreateFormFile(file.Field, file.Filename)
		if err != nil {
			t.Fatalf("create form file: %v", err)
		}
		if _, err := fw.Write(file.Content); err != nil {
			t.Fatalf("write file: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, baseURL+path, &buf)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	return sendDecode(t, req)
}

func sendDecode(t *testing.T, req *http.Request) (int, map[string]any) {
	t.Helper()
	resp, err := httpClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", req.Method, req.URL.Path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	var m map[string]any
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatalf("%s %s: response is not a JSON object (status %d): %s",
				req.Method, req.URL.Path, resp.StatusCode, truncate(raw))
		}
	}
	return resp.StatusCode, m
}

// getRaw fetches a path and returns the response (for headers) and the body as a string
// (SPA HTML / header assertions).
func getRaw(t *testing.T, path string) (*http.Response, string) {
	t.Helper()
	resp, err := httpClient.Get(baseURL + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	return resp, string(raw)
}

func truncate(b []byte) string {
	if len(b) > 300 {
		return string(b[:300]) + "..."
	}
	return string(b)
}

// --- assertion + getter helpers (stdlib only) ---

func mustStatus(t *testing.T, got, want int) {
	t.Helper()
	if got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
}

func mustContains(t *testing.T, s, sub string) {
	t.Helper()
	if !strings.Contains(s, sub) {
		t.Errorf("expected %q to contain %q", truncate([]byte(s)), sub)
	}
}

func mustNotContains(t *testing.T, s, sub string) {
	t.Helper()
	if strings.Contains(s, sub) {
		t.Errorf("expected %q NOT to contain %q", truncate([]byte(s)), sub)
	}
}

func has(m map[string]any, key string) bool {
	_, ok := m[key]
	return ok
}

func isNull(m map[string]any, key string) bool {
	v, ok := m[key]
	return ok && v == nil
}

func gstr(t *testing.T, m map[string]any, key string) string {
	t.Helper()
	v, ok := m[key]
	if !ok {
		t.Fatalf("missing key %q in %v", key, m)
	}
	s, ok := v.(string)
	if !ok {
		t.Fatalf("key %q = %v (%T), want string", key, v, v)
	}
	return s
}

func gnum(t *testing.T, m map[string]any, key string) float64 {
	t.Helper()
	v, ok := m[key]
	if !ok {
		t.Fatalf("missing key %q in %v", key, m)
	}
	n, ok := v.(float64)
	if !ok {
		t.Fatalf("key %q = %v (%T), want number", key, v, v)
	}
	return n
}

func gint(t *testing.T, m map[string]any, key string) int {
	t.Helper()
	return int(gnum(t, m, key))
}

func gbool(t *testing.T, m map[string]any, key string) bool {
	t.Helper()
	v, ok := m[key]
	if !ok {
		t.Fatalf("missing key %q in %v", key, m)
	}
	b, ok := v.(bool)
	if !ok {
		t.Fatalf("key %q = %v (%T), want bool", key, v, v)
	}
	return b
}

func gmap(t *testing.T, m map[string]any, key string) map[string]any {
	t.Helper()
	v, ok := m[key]
	if !ok {
		t.Fatalf("missing key %q in %v", key, m)
	}
	sub, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("key %q = %v (%T), want object", key, v, v)
	}
	return sub
}

func glist(t *testing.T, m map[string]any, key string) []any {
	t.Helper()
	v, ok := m[key]
	if !ok {
		t.Fatalf("missing key %q in %v", key, m)
	}
	l, ok := v.([]any)
	if !ok {
		t.Fatalf("key %q = %v (%T), want array", key, v, v)
	}
	return l
}

func gelem(t *testing.T, list []any, i int) map[string]any {
	t.Helper()
	if i >= len(list) {
		t.Fatalf("index %d out of range (len %d)", i, len(list))
	}
	m, ok := list[i].(map[string]any)
	if !ok {
		t.Fatalf("list[%d] = %v (%T), want object", i, list[i], list[i])
	}
	return m
}

// mustEqStr/Num/Bool assert a JSON object field equals an expected value.
func mustEqStr(t *testing.T, m map[string]any, key, want string) {
	t.Helper()
	if got := gstr(t, m, key); got != want {
		t.Errorf("%q = %q, want %q", key, got, want)
	}
}

func mustEqNum(t *testing.T, m map[string]any, key string, want float64) {
	t.Helper()
	if got := gnum(t, m, key); got != want {
		t.Errorf("%q = %v, want %v", key, got, want)
	}
}

func mustEqBool(t *testing.T, m map[string]any, key string, want bool) {
	t.Helper()
	if got := gbool(t, m, key); got != want {
		t.Errorf("%q = %v, want %v", key, got, want)
	}
}

func mustNull(t *testing.T, m map[string]any, key string) {
	t.Helper()
	if !has(m, key) {
		t.Errorf("missing key %q (want null)", key)
	} else if !isNull(m, key) {
		t.Errorf("%q = %v, want null", key, m[key])
	}
}

func mustHave(t *testing.T, m map[string]any, key string) {
	t.Helper()
	if !has(m, key) {
		t.Errorf("missing key %q", key)
	}
}
