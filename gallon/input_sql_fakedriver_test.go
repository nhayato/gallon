package gallon

// A tiny in-memory database/sql driver used only by tests in this package.
//
// It exists so that the keyset pagination page-advancing logic in
// InputPluginSql.Extract can be exercised deterministically end-to-end
// (including simulating a row being inserted into the source table *while*
// extraction is in progress), without requiring a real database connection
// (docker) or an external mocking dependency such as sqlmock.
//
// It intentionally supports only the small set of query shapes that
// buildKeysetPagedQueries produces:
//   - "SELECT * FROM <t> ORDER BY <key> LIMIT ?"            (first page)
//   - "SELECT * FROM <t> WHERE <key> > ? ORDER BY <key> LIMIT ?" (next page)

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
)

// fakeTable is an in-memory table shared by every connection opened against
// the same DSN. rows may be mutated concurrently with an in-flight Extract()
// call (via onQuery) to simulate inserts happening during extraction.
type fakeTable struct {
	mu      sync.Mutex
	columns []string
	rows    [][]any

	// queryCallCount counts how many times Query has been executed against
	// this table (across both the first-page and next-page prepared
	// statements), so tests can inject a mutation at a specific point in the
	// pagination sequence (e.g. "right before the 2nd page is fetched").
	queryCallCount int
	// onQuery, if set, is invoked with the 0-indexed call number before the
	// query for that call is evaluated against rows.
	onQuery func(callIndex int, tbl *fakeTable)
}

func (t *fakeTable) insertLocked(row []any) {
	t.rows = append(t.rows, row)
}

var (
	fakeTablesMu sync.Mutex
	fakeTables   = map[string]*fakeTable{}
	fakeDSNSeq   int
)

// newFakeSqlDB registers a new fake table under a fresh, unique DSN and
// returns a *sql.DB connected to it via the fake driver.
func newFakeSqlDB(columns []string, rows [][]any, onQuery func(callIndex int, tbl *fakeTable)) (*sql.DB, *fakeTable, error) {
	fakeTablesMu.Lock()
	fakeDSNSeq++
	dsn := fmt.Sprintf("fake-%d", fakeDSNSeq)
	tbl := &fakeTable{columns: columns, rows: rows, onQuery: onQuery}
	fakeTables[dsn] = tbl
	fakeTablesMu.Unlock()

	db, err := sql.Open("gallon_fake_sql_test_driver", dsn)
	if err != nil {
		return nil, nil, err
	}

	return db, tbl, nil
}

func init() {
	sql.Register("gallon_fake_sql_test_driver", &fakeSqlDriver{})
}

type fakeSqlDriver struct{}

func (d *fakeSqlDriver) Open(name string) (driver.Conn, error) {
	fakeTablesMu.Lock()
	tbl, ok := fakeTables[name]
	fakeTablesMu.Unlock()
	if !ok {
		return nil, fmt.Errorf("fakeSqlDriver: unknown dsn: %v", name)
	}

	return &fakeConn{table: tbl}, nil
}

type fakeConn struct {
	table *fakeTable
}

func (c *fakeConn) Prepare(query string) (driver.Stmt, error) {
	return &fakeStmt{table: c.table, query: query}, nil
}

func (c *fakeConn) Close() error { return nil }

func (c *fakeConn) Begin() (driver.Tx, error) {
	return nil, fmt.Errorf("fakeConn: transactions are not supported")
}

type fakeStmt struct {
	table *fakeTable
	query string
}

func (s *fakeStmt) Close() error  { return nil }
func (s *fakeStmt) NumInput() int { return -1 } // let database/sql skip arg-count validation

func (s *fakeStmt) Exec(args []driver.Value) (driver.Result, error) {
	return nil, fmt.Errorf("fakeStmt: Exec is not supported")
}

func (s *fakeStmt) Query(args []driver.Value) (driver.Rows, error) {
	s.table.mu.Lock()
	defer s.table.mu.Unlock()

	callIndex := s.table.queryCallCount
	s.table.queryCallCount++

	if s.table.onQuery != nil {
		s.table.onQuery(callIndex, s.table)
	}

	snapshot := make([][]any, len(s.table.rows))
	copy(snapshot, s.table.rows)
	sort.Slice(snapshot, func(i, j int) bool {
		return compareFakeKey(snapshot[i][0], snapshot[j][0]) < 0
	})

	var filtered [][]any
	var limit int64

	switch {
	case strings.Contains(s.query, "WHERE"):
		// next-page keyset query: WHERE <key> > ? ORDER BY <key> LIMIT ?
		lastKey := args[0]
		limitArg, ok := args[1].(int64)
		if !ok {
			return nil, fmt.Errorf("fakeStmt: expected int64 limit, got %T", args[1])
		}
		limit = limitArg

		for _, row := range snapshot {
			if compareFakeKey(row[0], lastKey) > 0 {
				filtered = append(filtered, row)
			}
		}
	case strings.Contains(s.query, "ORDER BY"):
		// first-page keyset query: ORDER BY <key> LIMIT ?
		limitArg, ok := args[0].(int64)
		if !ok {
			return nil, fmt.Errorf("fakeStmt: expected int64 limit, got %T", args[0])
		}
		limit = limitArg
		filtered = snapshot
	default:
		return nil, fmt.Errorf("fakeStmt: unsupported query shape: %v", s.query)
	}

	if int64(len(filtered)) > limit {
		filtered = filtered[:limit]
	}

	values := make([][]driver.Value, len(filtered))
	for i, row := range filtered {
		dv := make([]driver.Value, len(row))
		for j, v := range row {
			dv[j] = v
		}
		values[i] = dv
	}

	return &fakeRows{columns: s.table.columns, rows: values}, nil
}

// compareFakeKey compares two pagination-key values of the same underlying
// type (int64 or string are supported, which is sufficient for the query
// shapes under test).
func compareFakeKey(a, b any) int {
	switch av := a.(type) {
	case int64:
		bv := b.(int64)
		switch {
		case av < bv:
			return -1
		case av > bv:
			return 1
		default:
			return 0
		}
	case string:
		return strings.Compare(av, b.(string))
	default:
		panic(fmt.Sprintf("compareFakeKey: unsupported key type %T", a))
	}
}

type fakeRows struct {
	columns []string
	rows    [][]driver.Value
	idx     int
}

func (r *fakeRows) Columns() []string { return r.columns }
func (r *fakeRows) Close() error      { return nil }

func (r *fakeRows) Next(dest []driver.Value) error {
	if r.idx >= len(r.rows) {
		return io.EOF
	}

	copy(dest, r.rows[r.idx])
	r.idx++

	return nil
}
