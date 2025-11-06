// Package mysql provides the MySQL-backed database plugin implementation.
package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/lcx/asura/db"
)

var (
	// sqlOpenFn wraps sql.Open so tests can replace it with a stub implementation.
	sqlOpenFn = sql.Open
	// mysqlDriverName stores the sql driver identifier; tests may override this value.
	mysqlDriverName = "mysql"
)

// Config contains the connection settings used to initialise a MySQL database handle.
type Config struct {
	Tag         string `mapstructure:"tag"`
	DataSource  string `mapstructure:"datasource"`
	IdleConns   int    `mapstructure:"idleconns"`
	MaxLifeTime uint32 `mapstructure:"maxlifetime"`
}

// DB wraps a single logical connection pool to a MySQL cluster.
type DB struct {
	db     *sql.DB
	ctx    context.Context
	cancel context.CancelFunc
}

// DBOpen creates and initialises a DB instance using the provided configuration.
func DBOpen(cfg *Config) (db.Database, error) {
	d := &DB{}
	if e := d.Open(cfg); e != nil {
		return nil, e
	}
	return d, nil
}

// FactoryName returns the plugin factory identifier.
func (t *DB) FactoryName() string {
	return "mysql"
}

// Open configures the DB using the supplied configuration payload.
func (t *DB) Open(c any) error {
	cfg, ok := c.(*Config)
	if !ok {
		return errors.New("config type not TcaplusDBCfg")
	}

	db, err := sqlOpenFn(mysqlDriverName, cfg.DataSource)
	if err != nil {
		return fmt.Errorf("failed to open mysql, datasource: %s, err: %w", cfg.DataSource, err)
	}
	db.SetMaxIdleConns(cfg.IdleConns)
	db.SetConnMaxLifetime(time.Second * time.Duration(cfg.MaxLifeTime))

	if err = db.Ping(); err != nil {
		return fmt.Errorf("failed to ping mysql, datasource: %s, err: %w", cfg.DataSource, err)
	}
	t.db = db
	t.ctx, t.cancel = context.WithCancel(context.Background())
	return nil
}

// OnTick performs periodic maintenance; MySQL implementation currently has no background work.
func (t *DB) OnTick() {}

// Get retrieves a record by primary key and unmarshals the result into record.
func (t *DB) Get(record db.Record, fields []db.Field) error {
	fs := db.FieldsToStrings(fields)
	rf := record.ProtoReflect()
	meta := GetDBProtoMeta(rf.Descriptor())
	if meta == nil {
		return fmt.Errorf("get meta nil: fullname=%s", string(rf.Descriptor().FullName()))
	}
	sqlpkg := meta.SelectFieldsPkg(rf, fs)
	rows, err := t.db.QueryContext(t.ctx, sqlpkg.str, sqlpkg.params...)
	if err != nil {
		return err
	}
	if rows.Err() != nil {
		return rows.Err()
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return err
	}

	// No matching row was found for the supplied primary key.
	if !rows.Next() {
		return db.ErrRecordNotExist
	}

	values := make([]sql.NullString, len(cols))
	args := make([]any, len(cols))
	for i := 0; i < len(cols); i++ {
		args[i] = &values[i]
	}
	if e := rows.Scan(args...); e != nil {
		return fmt.Errorf("result scan: %w", e)
	}
	retMap := map[string]string{}
	for i := 0; i < len(cols); i++ {
		retMap[cols[i]] = values[i].String
	}

	return db.UnmarshalFromMap(record, retMap)
}

// Update persists changes for the supplied record.
func (t *DB) Update(record db.Record, fields []db.Field) error {
	rf := record.ProtoReflect()
	fs := db.FieldsToStrings(fields)
	m, err := db.MarshalToMap(record, fs)
	if err != nil {
		return fmt.Errorf("marshal map failed: %w", err)
	}
	meta := GetDBProtoMeta(rf.Descriptor())
	if meta == nil {
		return fmt.Errorf("get meta nil: fullname=%s", string(rf.Descriptor().FullName()))
	}
	sqlpkg := meta.UpdatePkg(rf, m)
	if sqlpkg == nil {
		return fmt.Errorf("build sql: fullname=%s", string(rf.Descriptor().FullName()))
	}
	r, e := t.db.ExecContext(t.ctx, sqlpkg.str, sqlpkg.params...)
	if e != nil {
		return fmt.Errorf("mysql exec: %w", e)
	}
	n, e := r.RowsAffected()
	if e != nil {
		return fmt.Errorf("mysql row affect: %w", e)
	}
	if n == 0 {
		return fmt.Errorf("affect 0")
	}
	return nil
}

// Insert stores a new record and fails if the primary key already exists.
func (t *DB) Insert(record db.Record) error {
	rf := record.ProtoReflect()
	m, err := db.MarshalToMap(record, nil)
	if err != nil {
		return fmt.Errorf("marshal map failed: %w", err)
	}
	meta := GetDBProtoMeta(rf.Descriptor())
	if meta == nil {
		return fmt.Errorf("get meta nil: fullname=%s", string(rf.Descriptor().FullName()))
	}
	sqlpkg := meta.InsertPkg(m)
	if sqlpkg == nil {
		return fmt.Errorf("build sql: fullname=%s", string(rf.Descriptor().FullName()))
	}
	_, e := t.db.ExecContext(t.ctx, sqlpkg.str, sqlpkg.params...)
	if e != nil {
		return fmt.Errorf("mysql exec: %w", e)
	}
	return nil
}

// Replace upserts the record, creating it if it does not already exist.
func (t *DB) Replace(record db.Record, _ []db.Field) error {
	rf := record.ProtoReflect()
	m, err := db.MarshalToMap(record, nil)
	if err != nil {
		return fmt.Errorf("marshal map failed: %w", err)
	}
	meta := GetDBProtoMeta(rf.Descriptor())
	if meta == nil {
		return fmt.Errorf("get meta nil: fullname=%s", string(rf.Descriptor().FullName()))
	}

	sqlpkg := meta.ReplacePkg(m)
	_, e := t.db.ExecContext(t.ctx, sqlpkg.str, sqlpkg.params...)
	if e != nil {
		return fmt.Errorf("mysql exec: %w", e)
	}
	return nil
}

// Delete removes the record addressed by the primary key fields.
func (t *DB) Delete(record db.Record) error {
	rf := record.ProtoReflect()
	meta := GetDBProtoMeta(rf.Descriptor())
	if meta == nil {
		return fmt.Errorf("mysql cannt find record descriptor")
	}

	sqlpkg := meta.DeleteSQLPkg(rf)
	_, e := t.db.ExecContext(t.ctx, sqlpkg.str, sqlpkg.params...)
	if e != nil {
		return fmt.Errorf("mysql exec: %w", e)
	}
	return nil
}

// Increase increments the supplied numeric fields atomically.
func (t *DB) Increase(record db.Record, fields []db.Field) error {
	rf := record.ProtoReflect()
	fs := db.FieldsToStrings(fields)
	meta := GetDBProtoMeta(rf.Descriptor())
	if meta == nil {
		return fmt.Errorf("get meta nil: fullname=%s", string(rf.Descriptor().FullName()))
	}

	sqlpkg := meta.IncreasePkg(rf, fs)
	if sqlpkg == nil {
		return fmt.Errorf("increase sql build: fullname=%s", string(rf.Descriptor().FullName()))
	}
	_, e := t.db.ExecContext(t.ctx, sqlpkg.str, sqlpkg.params...)
	if e != nil {
		return fmt.Errorf("mysql exec: %w", e)
	}
	return nil
}
