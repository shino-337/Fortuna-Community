package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// NullTime is a *time.Time that scans from string (e.g. SQLite datetime) or time.Time.
// Use for Pod.StartTime so tests with SQLite work.
type NullTime struct {
	Time *time.Time
}

// Scan implements sql.Scanner.
func (n *NullTime) Scan(value interface{}) error {
	if value == nil {
		n.Time = nil
		return nil
	}
	switch v := value.(type) {
	case time.Time:
		t := v
		n.Time = &t
		return nil
	case string:
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			t, err = time.Parse("2006-01-02 15:04:05.999999999-07:00", v)
		}
		if err != nil {
			t, err = time.Parse("2006-01-02 15:04:05Z07:00", v)
		}
		if err != nil {
			return err
		}
		n.Time = &t
		return nil
	case []byte:
		return n.Scan(string(v))
	default:
		return fmt.Errorf("cannot scan %T into NullTime", value)
	}
}

// Value implements driver.Valuer.
func (n NullTime) Value() (driver.Value, error) {
	if n.Time == nil {
		return nil, nil
	}
	return *n.Time, nil
}

// MarshalJSON implements json.Marshaler.
func (n NullTime) MarshalJSON() ([]byte, error) {
	if n.Time == nil {
		return []byte("null"), nil
	}
	return json.Marshal(n.Time)
}

// UnmarshalJSON implements json.Unmarshaler.
func (n *NullTime) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		n.Time = nil
		return nil
	}
	var t time.Time
	if err := json.Unmarshal(data, &t); err != nil {
		return err
	}
	n.Time = &t
	return nil
}
