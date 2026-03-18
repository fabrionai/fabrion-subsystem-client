package airbyte

import (
	"encoding/json"
	"time"
)

// FlexibleTime handles timestamps with or without seconds.
// Use this type instead of time.Time in generated models for robust parsing.
// Example: 2025-12-16T09:36Z or 2025-12-16T09:36:00Z
type FlexibleTime struct {
	Time time.Time
}

// UnmarshalJSON implements the json.Unmarshaler interface for FlexibleTime.
// Handles null, empty, and various timestamp formats robustly.
func (ft *FlexibleTime) UnmarshalJSON(b []byte) error {
	s := string(b)
	if s == "null" || len(s) < 2 {
		return nil
	}
	if s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	} else {
		return nil
	}
	var layouts = []string{
		time.RFC3339,
		"2006-01-02T15:04-07:00",
		"2006-01-02T15:04Z",
	}
	var err error
	for _, layout := range layouts {
		ft.Time, err = time.Parse(layout, s)
		if err == nil {
			return nil
		}
	}
	return err
}

// MarshalJSON implements the json.Marshaler interface for FlexibleTime.
func (ft FlexibleTime) MarshalJSON() ([]byte, error) {
	return json.Marshal(ft.Time.Format(time.RFC3339))
}

// String returns the string representation of FlexibleTime.
func (ft FlexibleTime) String() string {
	return ft.Time.String()
}
