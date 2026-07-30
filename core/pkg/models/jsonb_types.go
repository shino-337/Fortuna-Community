package models

import (
	"database/sql/driver"
	"encoding/json"
)

// JSONBStringArray is []string that scans from PostgreSQL JSONB array and marshals to JSONB.
// Use for columns defined as JSONB storing JSON arrays of strings (e.g. ["a","b"]).
// Do not use pq.StringArray for JSONB columns; pq.StringArray expects native PostgreSQL text[].
type JSONBStringArray []string

func (a *JSONBStringArray) Scan(value interface{}) error {
	if value == nil {
		*a = nil
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		*a = []string{}
		return nil
	}
	if len(bytes) == 0 || string(bytes) == "null" {
		*a = []string{}
		return nil
	}
	var arr []string
	if err := json.Unmarshal(bytes, &arr); err != nil {
		*a = []string{}
		return nil
	}
	*a = arr
	return nil
}

func (a JSONBStringArray) Value() (driver.Value, error) {
	if a == nil {
		return json.Marshal([]string{})
	}
	return json.Marshal([]string(a))
}
