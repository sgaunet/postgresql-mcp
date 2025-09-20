package app

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDatabaseInfoSerialization(t *testing.T) {
	db := &DatabaseInfo{
		Name:     "testdb",
		Owner:    "testuser",
		Encoding: "UTF8",
		Size:     "10MB",
	}

	// Test JSON serialization
	jsonData, err := json.Marshal(db)
	assert.NoError(t, err)
	assert.Contains(t, string(jsonData), "testdb")
	assert.Contains(t, string(jsonData), "testuser")
	assert.Contains(t, string(jsonData), "UTF8")
	assert.Contains(t, string(jsonData), "10MB")

	// Test JSON deserialization
	var deserializedDB DatabaseInfo
	err = json.Unmarshal(jsonData, &deserializedDB)
	assert.NoError(t, err)
	assert.Equal(t, db.Name, deserializedDB.Name)
	assert.Equal(t, db.Owner, deserializedDB.Owner)
	assert.Equal(t, db.Encoding, deserializedDB.Encoding)
	assert.Equal(t, db.Size, deserializedDB.Size)
}

func TestDatabaseInfoWithOmitEmpty(t *testing.T) {
	// Test with empty size (should be omitted)
	db := &DatabaseInfo{
		Name:     "testdb",
		Owner:    "testuser",
		Encoding: "UTF8",
	}

	jsonData, err := json.Marshal(db)
	assert.NoError(t, err)
	assert.Contains(t, string(jsonData), "testdb")
	assert.NotContains(t, string(jsonData), "size")
}

func TestSchemaInfoSerialization(t *testing.T) {
	schema := &SchemaInfo{
		Name:  "public",
		Owner: "postgres",
	}

	jsonData, err := json.Marshal(schema)
	assert.NoError(t, err)
	assert.Contains(t, string(jsonData), "public")
	assert.Contains(t, string(jsonData), "postgres")

	var deserializedSchema SchemaInfo
	err = json.Unmarshal(jsonData, &deserializedSchema)
	assert.NoError(t, err)
	assert.Equal(t, schema.Name, deserializedSchema.Name)
	assert.Equal(t, schema.Owner, deserializedSchema.Owner)
}

func TestTableInfoSerialization(t *testing.T) {
	table := &TableInfo{
		Schema:      "public",
		Name:        "users",
		Type:        "table",
		RowCount:    1000,
		Size:        "5MB",
		Owner:       "appuser",
		Description: "User accounts table",
	}

	jsonData, err := json.Marshal(table)
	assert.NoError(t, err)
	assert.Contains(t, string(jsonData), "public")
	assert.Contains(t, string(jsonData), "users")
	assert.Contains(t, string(jsonData), "table")
	assert.Contains(t, string(jsonData), "1000")
	assert.Contains(t, string(jsonData), "5MB")
	assert.Contains(t, string(jsonData), "appuser")
	assert.Contains(t, string(jsonData), "User accounts table")

	var deserializedTable TableInfo
	err = json.Unmarshal(jsonData, &deserializedTable)
	assert.NoError(t, err)
	assert.Equal(t, table.Schema, deserializedTable.Schema)
	assert.Equal(t, table.Name, deserializedTable.Name)
	assert.Equal(t, table.Type, deserializedTable.Type)
	assert.Equal(t, table.RowCount, deserializedTable.RowCount)
	assert.Equal(t, table.Size, deserializedTable.Size)
	assert.Equal(t, table.Owner, deserializedTable.Owner)
	assert.Equal(t, table.Description, deserializedTable.Description)
}

func TestTableInfoWithOmitEmpty(t *testing.T) {
	// Test with minimal fields (omitempty should work)
	table := &TableInfo{
		Schema: "public",
		Name:   "simple_table",
		Type:   "table",
		Owner:  "user",
	}

	jsonData, err := json.Marshal(table)
	assert.NoError(t, err)
	assert.Contains(t, string(jsonData), "public")
	assert.Contains(t, string(jsonData), "simple_table")
	assert.NotContains(t, string(jsonData), "row_count")
	assert.NotContains(t, string(jsonData), "size")
	assert.NotContains(t, string(jsonData), "description")
}

func TestColumnInfoSerialization(t *testing.T) {
	column := &ColumnInfo{
		Name:         "email",
		DataType:     "varchar(255)",
		IsNullable:   false,
		DefaultValue: "",
		Description:  "User email address",
	}

	jsonData, err := json.Marshal(column)
	assert.NoError(t, err)
	assert.Contains(t, string(jsonData), "email")
	assert.Contains(t, string(jsonData), "varchar(255)")
	assert.Contains(t, string(jsonData), "false")
	assert.Contains(t, string(jsonData), "User email address")

	var deserializedColumn ColumnInfo
	err = json.Unmarshal(jsonData, &deserializedColumn)
	assert.NoError(t, err)
	assert.Equal(t, column.Name, deserializedColumn.Name)
	assert.Equal(t, column.DataType, deserializedColumn.DataType)
	assert.Equal(t, column.IsNullable, deserializedColumn.IsNullable)
	assert.Equal(t, column.DefaultValue, deserializedColumn.DefaultValue)
	assert.Equal(t, column.Description, deserializedColumn.Description)
}

func TestColumnInfoNullable(t *testing.T) {
	column := &ColumnInfo{
		Name:       "optional_field",
		DataType:   "text",
		IsNullable: true,
	}

	jsonData, err := json.Marshal(column)
	assert.NoError(t, err)
	assert.Contains(t, string(jsonData), "true")
}

func TestIndexInfoSerialization(t *testing.T) {
	index := &IndexInfo{
		Name:      "idx_users_email",
		Table:     "users",
		Columns:   []string{"email"},
		IsUnique:  true,
		IsPrimary: false,
		IndexType: "btree",
		Size:      "2MB",
	}

	jsonData, err := json.Marshal(index)
	assert.NoError(t, err)
	assert.Contains(t, string(jsonData), "idx_users_email")
	assert.Contains(t, string(jsonData), "users")
	assert.Contains(t, string(jsonData), "email")
	assert.Contains(t, string(jsonData), "btree")
	assert.Contains(t, string(jsonData), "2MB")

	var deserializedIndex IndexInfo
	err = json.Unmarshal(jsonData, &deserializedIndex)
	assert.NoError(t, err)
	assert.Equal(t, index.Name, deserializedIndex.Name)
	assert.Equal(t, index.Table, deserializedIndex.Table)
	assert.Equal(t, index.Columns, deserializedIndex.Columns)
	assert.Equal(t, index.IsUnique, deserializedIndex.IsUnique)
	assert.Equal(t, index.IsPrimary, deserializedIndex.IsPrimary)
	assert.Equal(t, index.IndexType, deserializedIndex.IndexType)
	assert.Equal(t, index.Size, deserializedIndex.Size)
}

func TestIndexInfoMultipleColumns(t *testing.T) {
	index := &IndexInfo{
		Name:      "idx_users_name_email",
		Table:     "users",
		Columns:   []string{"last_name", "first_name", "email"},
		IsUnique:  false,
		IsPrimary: false,
		IndexType: "btree",
	}

	jsonData, err := json.Marshal(index)
	assert.NoError(t, err)

	var deserializedIndex IndexInfo
	err = json.Unmarshal(jsonData, &deserializedIndex)
	assert.NoError(t, err)
	assert.Len(t, deserializedIndex.Columns, 3)
	assert.Equal(t, []string{"last_name", "first_name", "email"}, deserializedIndex.Columns)
}

func TestPrimaryKeyIndex(t *testing.T) {
	index := &IndexInfo{
		Name:      "users_pkey",
		Table:     "users",
		Columns:   []string{"id"},
		IsUnique:  true,
		IsPrimary: true,
		IndexType: "btree",
	}

	jsonData, err := json.Marshal(index)
	assert.NoError(t, err)
	assert.Contains(t, string(jsonData), "true")

	var deserializedIndex IndexInfo
	err = json.Unmarshal(jsonData, &deserializedIndex)
	assert.NoError(t, err)
	assert.True(t, deserializedIndex.IsUnique)
	assert.True(t, deserializedIndex.IsPrimary)
}

func TestQueryResultSerialization(t *testing.T) {
	result := &QueryResult{
		Columns: []string{"id", "name", "email"},
		Rows: [][]interface{}{
			{1, "John Doe", "john@example.com"},
			{2, "Jane Smith", "jane@example.com"},
		},
		RowCount: 2,
	}

	jsonData, err := json.Marshal(result)
	assert.NoError(t, err)
	assert.Contains(t, string(jsonData), "id")
	assert.Contains(t, string(jsonData), "name")
	assert.Contains(t, string(jsonData), "email")
	assert.Contains(t, string(jsonData), "John Doe")
	assert.Contains(t, string(jsonData), "jane@example.com")

	var deserializedResult QueryResult
	err = json.Unmarshal(jsonData, &deserializedResult)
	assert.NoError(t, err)
	assert.Equal(t, result.Columns, deserializedResult.Columns)
	assert.Equal(t, result.RowCount, deserializedResult.RowCount)
	assert.Len(t, deserializedResult.Rows, 2)
}

func TestQueryResultEmpty(t *testing.T) {
	result := &QueryResult{
		Columns:  []string{"id", "name"},
		Rows:     [][]interface{}{},
		RowCount: 0,
	}

	jsonData, err := json.Marshal(result)
	assert.NoError(t, err)

	var deserializedResult QueryResult
	err = json.Unmarshal(jsonData, &deserializedResult)
	assert.NoError(t, err)
	assert.Equal(t, 0, deserializedResult.RowCount)
	assert.Len(t, deserializedResult.Rows, 0)
	assert.Len(t, deserializedResult.Columns, 2)
}

func TestQueryResultWithNullValues(t *testing.T) {
	result := &QueryResult{
		Columns: []string{"id", "optional_field"},
		Rows: [][]interface{}{
			{1, nil},
			{2, "value"},
		},
		RowCount: 2,
	}

	jsonData, err := json.Marshal(result)
	assert.NoError(t, err)

	var deserializedResult QueryResult
	err = json.Unmarshal(jsonData, &deserializedResult)
	assert.NoError(t, err)
	assert.Len(t, deserializedResult.Rows, 2)
	assert.Nil(t, deserializedResult.Rows[0][1])
	assert.Equal(t, "value", deserializedResult.Rows[1][1])
}

func TestQueryResultWithMixedTypes(t *testing.T) {
	result := &QueryResult{
		Columns: []string{"id", "name", "age", "active", "score"},
		Rows: [][]interface{}{
			{1, "John", 30, true, 95.5},
			{2, "Jane", 25, false, 87.2},
		},
		RowCount: 2,
	}

	jsonData, err := json.Marshal(result)
	assert.NoError(t, err)

	var deserializedResult QueryResult
	err = json.Unmarshal(jsonData, &deserializedResult)
	assert.NoError(t, err)
	assert.Equal(t, 2, deserializedResult.RowCount)

	// Note: JSON unmarshaling converts numbers to float64
	assert.Equal(t, float64(1), deserializedResult.Rows[0][0])
	assert.Equal(t, "John", deserializedResult.Rows[0][1])
	assert.Equal(t, float64(30), deserializedResult.Rows[0][2])
	assert.Equal(t, true, deserializedResult.Rows[0][3])
	assert.Equal(t, 95.5, deserializedResult.Rows[0][4])
}