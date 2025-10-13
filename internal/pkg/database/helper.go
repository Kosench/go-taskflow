package database

import (
	"database/sql"
	"encoding/json"
	"time"
)

type NullTime struct {
	sql.NullTime
}

func (nt NullTime) MarshalJSON() ([]byte, error) {
	if nt.Valid {
		return json.Marshal(nt.Time)
	}
	return []byte("null"), nil
}

// UnmarshalJSON implements json.Unmarshaler interface
func (nt *NullTime) UnmarshalJSON(data []byte) error {
	var t *time.Time
	if err := json.Unmarshal(data, &t); err != nil {
		return err
	}
	if t != nil {
		nt.Valid = true
		nt.Time = *t
	} else {
		nt.Valid = false
	}
	return nil
}

// NewNullTime creates a new NullTime
func NewNullTime(t *time.Time) NullTime {
	if t == nil {
		return NullTime{sql.NullTime{Valid: false}}
	}
	return NullTime{sql.NullTime{Time: *t, Valid: true}}
}

// NullString represents a sql.NullString that can be JSON marshaled
type NullString struct {
	sql.NullString
}

// MarshalJSON implements json.Marshaler interface
func (ns NullString) MarshalJSON() ([]byte, error) {
	if ns.Valid {
		return json.Marshal(ns.String)
	}
	return []byte("null"), nil
}

// UnmarshalJSON implements json.Unmarshaler interface
func (ns *NullString) UnmarshalJSON(data []byte) error {
	var s *string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	if s != nil {
		ns.Valid = true
		ns.String = *s
	} else {
		ns.Valid = false
	}
	return nil
}

// NewNullString creates a new NullString
func NewNullString(s string) NullString {
	if s == "" {
		return NullString{sql.NullString{Valid: false}}
	}
	return NullString{sql.NullString{String: s, Valid: true}}
}
