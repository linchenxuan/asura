// Package db defines the core database abstractions and helpers shared by the
// storage plugins.
package db

import (
	"errors"
	"unsafe"

	"google.golang.org/protobuf/proto"
)

var ( //nolint:gochecknoglobals // exported error values are shared across plugins.
	// ErrRecordNotExist indicates that the requested record could not be found.
	ErrRecordNotExist = errors.New("RecordNotExistErr")
	// ErrRecordExist indicates that the record already exists and cannot be inserted.
	ErrRecordExist = errors.New("RecordAlreadyExist")
	// ErrNilAsyncHost indicates an unexpected nil asynchronous host reference.
	ErrNilAsyncHost = errors.New("ErrNilAsyncHost")
	// ErrNoMoreRecord indicates that there are no more records to consume.
	ErrNoMoreRecord = errors.New("ErrNoMoreRecord")
	// ErrFetchNil indicates that no record was fetched when one was expected.
	ErrFetchNil = errors.New("ErrFetchNil")
)

// Record aliases the protobuf message interface used by database plugins.
type Record = proto.Message

// Field represents a logical database column that can be referenced in queries.
type Field string

// Index names a database index attached to a record definition.
type Index string

// RecordResultTable stores a collection of records returned alongside a status
// code.
type RecordResultTable struct {
	code    int32
	Records []Record
}

// Database describes a plugin capable of executing database operations.
type Database interface {
	DBExecutor
	// Open prepares the database implementation using the provided configuration.
	//  @param config implementation-specific configuration payload
	//  @return error if the configuration is invalid or initialization fails
	Open(config any) error

	// OnTick drives background work such as asynchronous fetches or result polling.
	OnTick()

	// FactoryName returns the factory identifier for the concrete implementation.
	FactoryName() string
}

// DBExecutor exposes a simplified CRUD interface. Implementations write results
// directly into the supplied model and intentionally omit advanced metadata such
// as error codes or match counts. Use DBResult when that metadata is required.
type DBExecutor interface {
	// Get fetches data into record. If fields is nil, the full record is retrieved.
	//  @param record target protobuf message with primary key populated
	//  @param fields optional subset of columns to fetch
	//  @return error on transport or decoding failures
	Get(record Record, fields []Field) error

	// Update persists changes to an existing record. If fields is nil, a full update
	// is performed.
	//  @param record message carrying the new values
	//  @param fields optional subset of columns to update
	Update(record Record, fields []Field) error

	// Replace updates the record or creates it if it does not already exist.
	//  @param record message carrying the new values
	//  @param fields optional subset of columns to update
	Replace(record Record, fields []Field) error

	// Insert creates a new record and fails if the record already exists.
	//  @param record message carrying the new values
	Insert(record Record) error

	// Delete removes the record identified by the primary key in record.
	//  @param record message with primary key set
	Delete(record Record) error

	// Increase atomically increments the specified numeric fields.
	//  @param record message containing the primary key
	//  @param fields collection of incrementable columns
	Increase(record Record, fields []Field) error
}

// DBRecord exposes result data for a single record returned by a query.
type DBRecord interface {
	// ErrCode returns the error code that applies to the individual record.
	ErrCode() int
	// GetIndex returns the index for the record, useful for list-based queries.
	GetIndex() int32
	// GetVersion returns the record version if the backend tracks optimistic locks.
	GetVersion() int32
	// GetData unmarshals the record into the provided message.
	GetData(m Record) error
}

// DBResult models the outcome of a database request that produces multiple records.
type DBResult interface {
	// ResultCode returns the backend-level error code for the request.
	ResultCode() int

	// FetchRecord iterates over records one at a time.
	FetchRecord() (record DBRecord, err error)

	// FetchData consumes the next record and unmarshals it into m. Any per-record
	// error is returned immediately.
	FetchData(m Record) error

	// FetchDataWithVersion behaves like FetchData and additionally returns the
	// record version where supported.
	FetchDataWithVersion(m Record) (version int32, err error)

	// FetchDataWithIdx behaves like FetchData and additionally returns the record
	// index where supported.
	FetchDataWithIdx(m Record) (index int32, err error)

	// RecordCount returns the number of records fetched in the current batch.
	RecordCount() int

	// MatchCount returns the total number of records that matched the query.
	MatchCount() int

	// SQLResult exposes the raw SQL aggregation results when supported.
	SQLResult() []string

	// AgentReqData returns the raw payload exchanged with the database agent.
	AgentReqData() []byte
}

// FieldsToStrings converts Field aliases into their string representation.
func FieldsToStrings(fields []Field) []string {
	if fields == nil {
		return nil
	}

	return *((*([]string))(unsafe.Pointer(&fields)))
}

// IndexsToStrings converts Index aliases into their string representation.
func IndexsToStrings(indexs []Index) []string {
	if indexs == nil {
		return nil
	}

	return *((*([]string))(unsafe.Pointer(&indexs)))
}

// StringsToIndexs wraps string names with the Index alias type.
func StringsToIndexs(indexs []string) []Index {
	if indexs == nil {
		return nil
	}

	return *((*([]Index))(unsafe.Pointer(&indexs)))
}
