// Package mysql implements table management helpers for the MySQL plugin.
package mysql

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/lcx/asura/db"
)

// CreateTable builds the table backing the supplied record. Intended for tests.
func (t *DB) CreateTable(record db.Record) error {
	tpl, err := template.New("CreateTable").Parse(_CreateTableTemplate)
	if err != nil {
		return err
	}
	ti, err := newTableInfo(record.ProtoReflect().Descriptor())
	if err != nil {
		return err
	}
	b := &bytes.Buffer{}
	err = tpl.Execute(b, ti)
	if err != nil {
		return err
	}
	sqlText := b.String()
	_, err = t.db.ExecContext(t.ctx, sqlText)
	return err
}

// DropTable removes the table derived from the record descriptor if it exists.
func (t *DB) DropTable(record db.Record) error {
	sqlText := fmt.Sprintf("drop table if exists %s", record.ProtoReflect().Descriptor().Name())
	if _, err := t.db.Exec(sqlText); err != nil {
		return err
	}
	return nil
}

// ExistTable reports whether the table for the record descriptor already exists.
func (t *DB) ExistTable(record db.Record) bool {
	sqlText := fmt.Sprintf("desc %s", record.ProtoReflect().Descriptor().Name())
	if _, err := t.db.ExecContext(t.ctx, sqlText); err != nil {
		return false
	}
	return true
}
