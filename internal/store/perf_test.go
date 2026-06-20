package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// --- statement-counting driver: wraps modernc to count Query/Exec round-trips ---

type queryCounter struct{ n int64 }

func (c *queryCounter) reset()      { atomic.StoreInt64(&c.n, 0) }
func (c *queryCounter) load() int64 { return atomic.LoadInt64(&c.n) }
func (c *queryCounter) add()        { atomic.AddInt64(&c.n, 1) }

type countingDriver struct {
	base    driver.Driver
	counter *queryCounter
}

func (d *countingDriver) Open(name string) (driver.Conn, error) {
	c, err := d.base.Open(name)
	if err != nil {
		return nil, err
	}
	return &countingConn{Conn: c, counter: d.counter}, nil
}

type countingConn struct {
	driver.Conn
	counter *queryCounter
}

func (c *countingConn) QueryContext(ctx context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	c.counter.add()
	return c.Conn.(driver.QueryerContext).QueryContext(ctx, q, args)
}

func (c *countingConn) ExecContext(ctx context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	c.counter.add()
	return c.Conn.(driver.ExecerContext).ExecContext(ctx, q, args)
}

var (
	countingOnce    sync.Once
	countingCounter = &queryCounter{}
)

// countingStore opens a migrated Store whose statements run through a shared counter. The
// read handle is pinned to one connection so counts are deterministic.
func countingStore(t *testing.T) (*Store, *queryCounter) {
	t.Helper()
	countingOnce.Do(func() {
		base, err := sql.Open("sqlite", ":memory:")
		if err != nil {
			panic(err)
		}
		drv := base.Driver()
		_ = base.Close()
		sql.Register("sqlite-counting", &countingDriver{base: drv, counter: countingCounter})
	})
	st, err := openWithDriver("sqlite-counting", filepath.Join(t.TempDir(), "perf.db"))
	if err != nil {
		t.Fatal(err)
	}
	st.read.SetMaxOpenConns(1)
	if err := Migrate(st.Write()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	// Warm the read connection so the first measured query pays no one-time setup.
	var dummy int
	_ = st.read.QueryRowContext(context.Background(), "SELECT 1").Scan(&dummy)
	return st, countingCounter
}

func measure(counter *queryCounter, fn func()) int64 {
	counter.reset()
	fn()
	return counter.load()
}

func seedJob(t *testing.T, st *Store, group, job string) {
	t.Helper()
	if _, err := st.UpsertJob(context.Background(),
		UpsertParams{GroupName: group, JobName: job, Status: "success"}, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
}

func TestPerfListJobsDefersBlobAndNoRedundantCount(t *testing.T) {
	st, counter := countingStore(t)
	ctx := context.Background()

	// The job column list references log_content ONLY as the derived has_log; the blob is
	// never selected (spec §5.1).
	if strings.Count(jobColumns, "log_content") != 1 || !strings.Contains(jobColumns, "(j.log_content IS NOT NULL)") {
		t.Errorf("jobColumns must select only the derived has_log, got: %s", jobColumns)
	}

	seedJob(t, st, "g", "j")

	if n := measure(counter, func() {
		if _, err := st.ListJobs(ctx, JobFilter{}); err != nil {
			t.Fatal(err)
		}
	}); n != 1 {
		t.Errorf("default ListJobs ran %d statements, want 1 (no redundant COUNT)", n)
	}

	limit := 10
	if n := measure(counter, func() {
		if _, err := st.ListJobs(ctx, JobFilter{Limit: &limit}); err != nil {
			t.Fatal(err)
		}
	}); n != 2 {
		t.Errorf("paginated ListJobs ran %d statements, want 2 (SELECT + COUNT)", n)
	}
}

func TestPerfHealthSingleAggregate(t *testing.T) {
	st, counter := countingStore(t)
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		seedJob(t, st, "g", "j"+strconv.Itoa(i))
	}
	if n := measure(counter, func() {
		if _, err := st.Health(ctx); err != nil {
			t.Fatal(err)
		}
	}); n != 1 {
		t.Errorf("Health ran %d statements, want 1 (single aggregate)", n)
	}
}

func TestPerfListGroupsN1Free(t *testing.T) {
	st, counter := countingStore(t)
	ctx := context.Background()

	for i := 0; i < 2; i++ {
		seedJob(t, st, "g"+strconv.Itoa(i), "j")
	}
	small := measure(counter, func() {
		if _, err := st.ListGroups(ctx); err != nil {
			t.Fatal(err)
		}
	})

	for i := 2; i < 20; i++ {
		seedJob(t, st, "g"+strconv.Itoa(i), "j")
	}
	large := measure(counter, func() {
		if _, err := st.ListGroups(ctx); err != nil {
			t.Fatal(err)
		}
	})

	if small != large {
		t.Errorf("ListGroups statements grew with group count: %d (2 groups) -> %d (20 groups) = N+1", small, large)
	}
	if large == 0 || large > 5 {
		t.Errorf("ListGroups ran %d statements, want a small constant (1..5)", large)
	}
}
