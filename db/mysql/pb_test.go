package mysql

import (
	"bytes"
	"strings"
	"testing"
	"text/template"

	"github.com/lcx/asura/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestNewTableInfo(t *testing.T) {
	resetMetaCache()

	desc := testRecordDescriptor(t)
	ti, err := newTableInfo(desc)
	require.NoError(t, err)

	require.Equal(t, "TestRecord", ti.TableName)
	require.Equal(t, "dbtest.TestRecord", ti.DescFullName)
	require.Equal(t, "id", ti.PrimaryKeys)
	require.Len(t, ti.CreateSQLs, 9)

	expectedDefinitions := map[string]struct{}{
		"`id` bigint NOT NULL DEFAULT 0":            {},
		"`name` text":                               {},
		"`counter` bigint NOT NULL DEFAULT 0":       {},
		"`hits` bigint UNSIGNED NOT NULL DEFAULT 0": {},
		"`active` tinyint NOT NULL DEFAULT 0":       {},
		"`score` double NOT NULL DEFAULT 0":         {},
		"`payload` blob":                            {},
		"`labels` blob":                             {},
		"`details` blob":                            {},
	}

	for _, actual := range ti.CreateSQLs {
		_, ok := expectedDefinitions[actual]
		assert.Truef(t, ok, "unexpected column definition: %s", actual)
		delete(expectedDefinitions, actual)
	}
	require.Empty(t, expectedDefinitions, "missing column definitions")
}

func TestCreateTableTemplate(t *testing.T) {
	resetMetaCache()

	desc := testRecordDescriptor(t)
	ti, err := newTableInfo(desc)
	require.NoError(t, err)

	tpl, err := template.New("CreateTable").Parse(_CreateTableTemplate)
	require.NoError(t, err)

	var buf bytes.Buffer
	require.NoError(t, tpl.Execute(&buf, ti))

	sql := buf.String()
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS TestRecord")
	require.Contains(t, sql, "`id` bigint NOT NULL DEFAULT 0")
	require.Contains(t, sql, "`payload` blob")
	require.Contains(t, sql, "PRIMARY KEY ( id )")
}

func TestGetDBProtoMetaCaching(t *testing.T) {
	resetMetaCache()

	desc := testRecordDescriptor(t)
	meta1 := GetDBProtoMeta(desc)
	require.NotNil(t, meta1)

	meta2 := GetDBProtoMeta(desc)
	require.Equal(t, meta1, meta2)
}

func TestSelectFieldsPkg(t *testing.T) {
	resetMetaCache()

	msg := newTestRecord(t)
	meta := GetDBProtoMeta(msg.Descriptor())

	pkgAll := meta.SelectFieldsPkg(msg, nil)
	require.Equal(t, "select * from TestRecord where id=?", pkgAll.str)
	require.Equal(t, []any{"42"}, pkgAll.params)

	pkgSubset := meta.SelectFieldsPkg(msg, []string{"name", "score"})
	require.Equal(t, "select name,score from TestRecord where id=?", pkgSubset.str)
	require.Equal(t, []any{"42"}, pkgSubset.params)
}

func TestUpdatePkg(t *testing.T) {
	resetMetaCache()

	msg := newTestRecord(t)
	fields := msg.Descriptor().Fields()
	msg.Set(fields.ByName("name"), protoreflect.ValueOfString("updated"))
	msg.Set(fields.ByName("counter"), protoreflect.ValueOfInt64(99))

	meta := GetDBProtoMeta(msg.Descriptor())
	updateMap, err := db.MarshalToMap(msg, []string{"name", "counter"})
	require.NoError(t, err)

	pkg := meta.UpdatePkg(msg, updateMap)
	require.NotNil(t, pkg)
	require.True(t, strings.HasPrefix(pkg.str, "update TestRecord set "))
	require.Contains(t, pkg.str, "name=?")
	require.Contains(t, pkg.str, "counter=?")
	require.True(t, strings.HasSuffix(pkg.str, " where id=?"))
	require.Len(t, pkg.params, 3)
	assert.ElementsMatch(t, []any{"updated", "99"}, pkg.params[:2])
	require.Equal(t, "42", pkg.params[2])
}

func TestReplacePkg(t *testing.T) {
	resetMetaCache()

	msg := newTestRecord(t)
	meta := GetDBProtoMeta(msg.Descriptor())

	m, err := db.MarshalToMap(msg, nil)
	require.NoError(t, err)

	pkg := meta.ReplacePkg(m)
	require.NotNil(t, pkg)
	require.True(t, strings.HasPrefix(pkg.str, "replace into TestRecord set "))
	require.Greater(t, len(pkg.params), 0)

	assignments := strings.TrimPrefix(pkg.str, "replace into TestRecord set ")
	for _, assignment := range strings.Split(assignments, ",") {
		require.Contains(t, assignment, "=?")
		parts := strings.SplitN(assignment, "=", 2)
		key := parts[0]
		value, ok := m[key]
		require.Truef(t, ok, "unexpected assignment: %s", assignment)
		require.NotEmpty(t, pkg.params)
		require.Equal(t, value, pkg.params[0])
		pkg.params = pkg.params[1:]
		delete(m, key)
	}
	require.Empty(t, m)
}

func TestInsertPkg(t *testing.T) {
	resetMetaCache()

	msg := newTestRecord(t)
	meta := GetDBProtoMeta(msg.Descriptor())

	m, err := db.MarshalToMap(msg, nil)
	require.NoError(t, err)

	pkg := meta.InsertPkg(m)
	require.NotNil(t, pkg)
	require.True(t, strings.HasPrefix(pkg.str, "insert into TestRecord ("))

	open := strings.Index(pkg.str, "(")
	close := strings.Index(pkg.str, ") values")
	require.Greater(t, open, 0)
	require.Greater(t, close, open)

	columns := strings.Split(pkg.str[open+1:close], ",")
	require.ElementsMatch(t, mapsKeys(m), columns)
	require.Equal(t, len(columns), len(pkg.params))

	for i, col := range columns {
		require.Equal(t, m[col], pkg.params[i])
	}
}

func TestIncreasePkg(t *testing.T) {
	resetMetaCache()

	msg := newTestRecord(t)
	meta := GetDBProtoMeta(msg.Descriptor())

	pkg := meta.IncreasePkg(msg, []string{"counter", "hits"})
	require.NotNil(t, pkg)
	require.Equal(t, "update TestRecord set counter=counter+1,hits=hits+1 where id=?", pkg.str)
	require.Equal(t, []any{"42"}, pkg.params)

	invalid := meta.IncreasePkg(msg, []string{"name"})
	require.Nil(t, invalid)
}

func TestDeleteSQLPkg(t *testing.T) {
	resetMetaCache()

	msg := newTestRecord(t)
	meta := GetDBProtoMeta(msg.Descriptor())

	pkg := meta.DeleteSQLPkg(msg)
	require.NotNil(t, pkg)
	require.Equal(t, "delete from TestRecord where id=?", pkg.str)
	require.Equal(t, []any{"42"}, pkg.params)
}

func mapsKeys(m map[string]any) []string {
	ret := make([]string, 0, len(m))
	for k := range m {
		ret = append(ret, k)
	}
	return ret
}
