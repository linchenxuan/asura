package mysql

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lcx/asura/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
)

type opType int

const (
	opQuery opType = iota
	opExec
)

type queryResult struct {
	columns []string
	rows    [][]driver.Value
}

type execResult struct {
	rowsAffected int64
}

type scriptedOp struct {
	typ         opType
	queryResult *queryResult
	execResult  *execResult
	err         error

	sql  string
	args []driver.Value
}

type dbScript struct {
	t       *testing.T
	pingErr error

	mu  sync.Mutex
	ops []*scriptedOp
	pos int
}

func newDBScript(t *testing.T, pingErr error, ops ...*scriptedOp) *dbScript {
	t.Helper()
	return &dbScript{
		t:       t,
		pingErr: pingErr,
		ops:     ops,
	}
}

func (s *dbScript) next(kind opType) *scriptedOp {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.pos >= len(s.ops) {
		s.t.Fatalf("unexpected %v: no scripted operation remaining", kind)
	}
	op := s.ops[s.pos]
	if op.typ != kind {
		s.t.Fatalf("expected scripted operation %v, got %v", op.typ, kind)
	}
	s.pos++
	return op
}

func (s *dbScript) assertExhausted() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pos != len(s.ops) {
		s.t.Fatalf("unused scripted operations: %d", len(s.ops)-s.pos)
	}
}

type scriptedConnector struct {
	script *dbScript
}

func (c *scriptedConnector) Connect(context.Context) (driver.Conn, error) {
	return &scriptedConn{script: c.script}, nil
}

func (c *scriptedConnector) Driver() driver.Driver {
	return &scriptedDriver{script: c.script}
}

type scriptedDriver struct {
	script *dbScript
}

func (d *scriptedDriver) Open(string) (driver.Conn, error) {
	return &scriptedConn{script: d.script}, nil
}

type scriptedConn struct {
	script *dbScript
}

func (c *scriptedConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepare not supported in scripted driver")
}

func (c *scriptedConn) Close() error {
	return nil
}

func (c *scriptedConn) Begin() (driver.Tx, error) {
	return &stubTx{}, nil
}

func (c *scriptedConn) Ping(context.Context) error {
	return c.script.pingErr
}

func (c *scriptedConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	op := c.script.next(opQuery)
	op.sql = query
	op.args = namedValuesToValues(args)
	if op.err != nil {
		return nil, op.err
	}
	if op.queryResult == nil {
		return nil, errors.New("query result not provided")
	}
	return &scriptedRows{
		columns: op.queryResult.columns,
		rows:    op.queryResult.rows,
	}, nil
}

func (c *scriptedConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	op := c.script.next(opExec)
	op.sql = query
	op.args = namedValuesToValues(args)
	if op.err != nil {
		return nil, op.err
	}
	if op.execResult == nil {
		return driver.RowsAffected(0), nil
	}
	return scriptedResult(op.execResult.rowsAffected), nil
}

type scriptedRows struct {
	columns []string
	rows    [][]driver.Value
	index   int
}

func (r *scriptedRows) Columns() []string {
	return r.columns
}

func (r *scriptedRows) Close() error {
	return nil
}

func (r *scriptedRows) Next(dest []driver.Value) error {
	if r.index >= len(r.rows) {
		return io.EOF
	}
	row := r.rows[r.index]
	for i := range dest {
		if i < len(row) {
			dest[i] = row[i]
		} else {
			dest[i] = nil
		}
	}
	r.index++
	return nil
}

type stubTx struct{}

func (t *stubTx) Commit() error   { return nil }
func (t *stubTx) Rollback() error { return nil }

type scriptedResult int64

func (r scriptedResult) LastInsertId() (int64, error) {
	return 0, errors.New("last insert id not supported")
}

func (r scriptedResult) RowsAffected() (int64, error) {
	return int64(r), nil
}

func namedValuesToValues(in []driver.NamedValue) []driver.Value {
	out := make([]driver.Value, len(in))
	for i, v := range in {
		out[i] = v.Value
	}
	return out
}

func newScriptDB(t *testing.T, script *dbScript) *sql.DB {
	t.Helper()
	dbConn := sql.OpenDB(&scriptedConnector{script: script})
	dbConn.SetMaxOpenConns(1)
	return dbConn
}

func TestDBOpenInvalidConfigType(t *testing.T) {
	var database DB
	require.EqualError(t, database.Open("invalid"), "config type not TcaplusDBCfg")
}

func TestDBOpenFailsWhenSqlOpenErrors(t *testing.T) {
	original := sqlOpenFn
	sqlOpenFn = func(string, string) (*sql.DB, error) {
		return nil, errors.New("dial failure")
	}
	t.Cleanup(func() { sqlOpenFn = original })

	cfg := &Config{
		Tag:         "unit",
		DataSource:  "dsn",
		IdleConns:   1,
		MaxLifeTime: 1,
	}

	var database DB
	err := database.Open(cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to open mysql")
}

func TestDBOpenFailsWhenPingErrors(t *testing.T) {
	original := sqlOpenFn
	script := newDBScript(t, errors.New("ping failure"))
	stubDB := newScriptDB(t, script)
	sqlOpenFn = func(string, string) (*sql.DB, error) {
		return stubDB, nil
	}
	t.Cleanup(func() {
		sqlOpenFn = original
		_ = stubDB.Close()
	})

	cfg := &Config{
		Tag:         "unit",
		DataSource:  "dsn",
		IdleConns:   1,
		MaxLifeTime: 1,
	}

	var database DB
	err := database.Open(cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to ping mysql")
}

func TestDBOpenSuccess(t *testing.T) {
	original := sqlOpenFn
	script := newDBScript(t, nil)
	stubDB := newScriptDB(t, script)
	sqlOpenFn = func(driverName, dataSource string) (*sql.DB, error) {
		require.Equal(t, mysqlDriverName, driverName)
		require.Equal(t, "dsn", dataSource)
		return stubDB, nil
	}
	t.Cleanup(func() {
		sqlOpenFn = original
		_ = stubDB.Close()
	})

	cfg := &Config{
		Tag:         "unit",
		DataSource:  "dsn",
		IdleConns:   3,
		MaxLifeTime: uint32((10 * time.Second).Seconds()),
	}

	var database DB
	require.NoError(t, database.Open(cfg))
	require.NotNil(t, database.db)
	require.NotNil(t, database.ctx)
	require.NotNil(t, database.cancel)

	database.cancel()
	require.NoError(t, database.db.Close())
}

type stubDatabase struct{}

func (d *stubDatabase) FactoryName() string                  { return "stub" }
func (d *stubDatabase) Open(any) error                       { return nil }
func (d *stubDatabase) OnTick()                              {}
func (d *stubDatabase) Get(db.Record, []db.Field) error      { return nil }
func (d *stubDatabase) Update(db.Record, []db.Field) error   { return nil }
func (d *stubDatabase) Replace(db.Record, []db.Field) error  { return nil }
func (d *stubDatabase) Insert(db.Record) error               { return nil }
func (d *stubDatabase) Delete(db.Record) error               { return nil }
func (d *stubDatabase) Increase(db.Record, []db.Field) error { return nil }

func TestDBGetSuccess(t *testing.T) {
	resetMetaCache()

	queryOp := &scriptedOp{
		typ: opQuery,
		queryResult: &queryResult{
			columns: []string{"id", "name", "counter"},
			rows: [][]driver.Value{
				{"42", "loaded", "99"},
			},
		},
	}
	script := newDBScript(t, nil, queryOp)
	stubDB := newScriptDB(t, script)
	defer stubDB.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	database := &DB{
		db:     stubDB,
		ctx:    ctx,
		cancel: cancel,
	}

	desc := testRecordDescriptor(t)
	record := dynamicpb.NewMessage(desc)
	record.Set(desc.Fields().ByName("id"), protoreflect.ValueOfInt64(42))

	require.NoError(t, database.Get(record, nil))

	require.Equal(t, "loaded", record.Get(desc.Fields().ByName("name")).String())
	require.Equal(t, "select * from TestRecord where id=?", queryOp.sql)
	require.Equal(t, []driver.Value{"42"}, queryOp.args)
	script.assertExhausted()
}

func TestDBGetNoRows(t *testing.T) {
	resetMetaCache()

	queryOp := &scriptedOp{
		typ: opQuery,
		queryResult: &queryResult{
			columns: []string{"id", "name"},
			rows:    [][]driver.Value{},
		},
	}
	script := newDBScript(t, nil, queryOp)
	stubDB := newScriptDB(t, script)
	defer stubDB.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	database := &DB{
		db:     stubDB,
		ctx:    ctx,
		cancel: cancel,
	}

	desc := testRecordDescriptor(t)
	record := dynamicpb.NewMessage(desc)
	record.Set(desc.Fields().ByName("id"), protoreflect.ValueOfInt64(42))

	err := database.Get(record, nil)
	require.ErrorIs(t, err, db.ErrRecordNotExist)
	require.Equal(t, "select * from TestRecord where id=?", queryOp.sql)
	script.assertExhausted()
}

func TestDBGetQueryError(t *testing.T) {
	resetMetaCache()

	queryOp := &scriptedOp{
		typ: opQuery,
		err: errors.New("query failed"),
	}
	script := newDBScript(t, nil, queryOp)
	stubDB := newScriptDB(t, script)
	defer stubDB.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	database := &DB{
		db:     stubDB,
		ctx:    ctx,
		cancel: cancel,
	}

	desc := testRecordDescriptor(t)
	record := dynamicpb.NewMessage(desc)
	record.Set(desc.Fields().ByName("id"), protoreflect.ValueOfInt64(42))

	err := database.Get(record, nil)
	require.EqualError(t, err, "query failed")
	require.Equal(t, "select * from TestRecord where id=?", queryOp.sql)
	script.assertExhausted()
}

func TestDBUpdateSuccess(t *testing.T) {
	resetMetaCache()

	execOp := &scriptedOp{
		typ:        opExec,
		execResult: &execResult{rowsAffected: 1},
	}
	script := newDBScript(t, nil, execOp)
	stubDB := newScriptDB(t, script)
	defer stubDB.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	database := &DB{
		db:     stubDB,
		ctx:    ctx,
		cancel: cancel,
	}

	record := newTestRecord(t)
	desc := record.Descriptor()
	record.Set(desc.Fields().ByName("name"), protoreflect.ValueOfString("updated"))
	record.Set(desc.Fields().ByName("counter"), protoreflect.ValueOfInt64(99))

	require.NoError(t, database.Update(record, []db.Field{"name", "counter"}))

	require.True(t, strings.HasPrefix(execOp.sql, "update TestRecord set "))
	require.Contains(t, execOp.sql, " where id=?")
	require.Len(t, execOp.args, 3)
	require.Equal(t, "42", execOp.args[2])
	assert.ElementsMatch(t, []driver.Value{"updated", "99"}, execOp.args[:2])
	script.assertExhausted()
}

func TestDBUpdateExecError(t *testing.T) {
	resetMetaCache()

	execOp := &scriptedOp{
		typ: opExec,
		err: errors.New("exec failed"),
	}
	script := newDBScript(t, nil, execOp)
	stubDB := newScriptDB(t, script)
	defer stubDB.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	database := &DB{
		db:     stubDB,
		ctx:    ctx,
		cancel: cancel,
	}

	record := newTestRecord(t)

	err := database.Update(record, []db.Field{"name"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "mysql exec")
	script.assertExhausted()
}

func TestDBUpdateZeroRows(t *testing.T) {
	resetMetaCache()

	execOp := &scriptedOp{
		typ:        opExec,
		execResult: &execResult{rowsAffected: 0},
	}
	script := newDBScript(t, nil, execOp)
	stubDB := newScriptDB(t, script)
	defer stubDB.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	database := &DB{
		db:     stubDB,
		ctx:    ctx,
		cancel: cancel,
	}

	record := newTestRecord(t)

	err := database.Update(record, []db.Field{"name"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "affect 0")
	script.assertExhausted()
}

func TestDBInsertSuccess(t *testing.T) {
	resetMetaCache()

	execOp := &scriptedOp{
		typ:        opExec,
		execResult: &execResult{rowsAffected: 1},
	}
	script := newDBScript(t, nil, execOp)
	stubDB := newScriptDB(t, script)
	defer stubDB.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	database := &DB{
		db:     stubDB,
		ctx:    ctx,
		cancel: cancel,
	}

	record := newTestRecord(t)
	require.NoError(t, database.Insert(record))

	require.True(t, strings.HasPrefix(execOp.sql, "insert into TestRecord"))
	require.Greater(t, len(execOp.args), 0)
	script.assertExhausted()
}

func TestDBInsertExecError(t *testing.T) {
	resetMetaCache()

	execOp := &scriptedOp{
		typ: opExec,
		err: errors.New("insert failed"),
	}
	script := newDBScript(t, nil, execOp)
	stubDB := newScriptDB(t, script)
	defer stubDB.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	database := &DB{
		db:     stubDB,
		ctx:    ctx,
		cancel: cancel,
	}

	record := newTestRecord(t)
	err := database.Insert(record)
	require.Error(t, err)
	require.Contains(t, err.Error(), "mysql exec")
	script.assertExhausted()
}

func TestDBReplaceSuccess(t *testing.T) {
	resetMetaCache()

	execOp := &scriptedOp{
		typ:        opExec,
		execResult: &execResult{rowsAffected: 1},
	}
	script := newDBScript(t, nil, execOp)
	stubDB := newScriptDB(t, script)
	defer stubDB.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	database := &DB{
		db:     stubDB,
		ctx:    ctx,
		cancel: cancel,
	}

	record := newTestRecord(t)
	require.NoError(t, database.Replace(record, nil))

	require.True(t, strings.HasPrefix(execOp.sql, "replace into TestRecord set"))
	require.Greater(t, len(execOp.args), 0)
	script.assertExhausted()
}

func TestDBDeleteSuccess(t *testing.T) {
	resetMetaCache()

	execOp := &scriptedOp{
		typ:        opExec,
		execResult: &execResult{rowsAffected: 1},
	}
	script := newDBScript(t, nil, execOp)
	stubDB := newScriptDB(t, script)
	defer stubDB.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	database := &DB{
		db:     stubDB,
		ctx:    ctx,
		cancel: cancel,
	}

	record := dynamicpb.NewMessage(testRecordDescriptor(t))
	record.Set(record.Descriptor().Fields().ByName("id"), protoreflect.ValueOfInt64(42))

	require.NoError(t, database.Delete(record))
	require.Equal(t, "delete from TestRecord where id=?", execOp.sql)
	require.Equal(t, []driver.Value{"42"}, execOp.args)
	script.assertExhausted()
}

func TestDBIncreaseSuccess(t *testing.T) {
	resetMetaCache()

	execOp := &scriptedOp{
		typ:        opExec,
		execResult: &execResult{rowsAffected: 1},
	}
	script := newDBScript(t, nil, execOp)
	stubDB := newScriptDB(t, script)
	defer stubDB.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	database := &DB{
		db:     stubDB,
		ctx:    ctx,
		cancel: cancel,
	}

	record := dynamicpb.NewMessage(testRecordDescriptor(t))
	record.Set(record.Descriptor().Fields().ByName("id"), protoreflect.ValueOfInt64(42))

	require.NoError(t, database.Increase(record, []db.Field{"counter", "hits"}))

	require.Equal(t, "update TestRecord set counter=counter+1,hits=hits+1 where id=?", execOp.sql)
	require.Equal(t, []driver.Value{"42"}, execOp.args)
	script.assertExhausted()
}

func TestDBIncreaseInvalidField(t *testing.T) {
	resetMetaCache()

	script := newDBScript(t, nil)
	stubDB := newScriptDB(t, script)
	defer stubDB.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	database := &DB{
		db:     stubDB,
		ctx:    ctx,
		cancel: cancel,
	}

	record := dynamicpb.NewMessage(testRecordDescriptor(t))
	record.Set(record.Descriptor().Fields().ByName("id"), protoreflect.ValueOfInt64(42))

	err := database.Increase(record, []db.Field{"name"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "increase sql build")
	script.assertExhausted()
}
