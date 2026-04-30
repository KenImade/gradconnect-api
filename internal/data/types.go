package data

import (
	"database/sql/driver"
	"fmt"
	"strings"
	"time"
)

type Date struct {
	time.Time
}

func (d *Date) UnmarshalJSON(data []byte) error {
	s := strings.Trim(string(data), `"`)
	if s == "" || s == "null" {
		return nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return fmt.Errorf("date must be in YYYY-MM-DD format: %w", err)
	}
	d.Time = t
	return nil
}

func (d Date) MarshalJSON() ([]byte, error) {
	if d.Time.IsZero() {
		return []byte("null"), nil
	}
	return []byte(`"` + d.Time.Format("2006-01-02") + `"`), nil
}

func (d *Date) Scan(value any) error {
	if value == nil {
		return nil
	}
	t, ok := value.(time.Time)
	if !ok {
		return fmt.Errorf("cannot scan %T into Date", value)
	}
	d.Time = t
	return nil
}

func (d Date) Value() (driver.Value, error) {
	if d.Time.IsZero() {
		return nil, nil
	}
	return d.Time.Format("2006-01-02"), nil
}

func (d Date) BeforeDate(other Date) bool {
	return d.Time.Before(other.Time)
}

func (d Date) EqualDate(other Date) bool {
	return d.Time.Equal(other.Time)
}

// ParseDate parses a YYYY-MM-DD string into a Date.
func ParseDate(s string) (Date, error) {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return Date{}, err
	}
	return Date{Time: t}, nil
}
