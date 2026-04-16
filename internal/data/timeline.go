package data

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type Timeline int32

var ErrInvalidTimelineFormat = errors.New("invalid timeline format")

func (t Timeline) MarshalJSON() ([]byte, error) {
	jsonValue := fmt.Sprintf("%d weeks", t)

	quotedJSONValue := strconv.Quote(jsonValue)

	return []byte(quotedJSONValue), nil
}

func (t *Timeline) UnmarshalJSON(jsonValue []byte) error {
	unquotedJSONValue, err := strconv.Unquote(string(jsonValue))
	if err != nil {
		return ErrInvalidTimelineFormat
	}

	parts := strings.Split(unquotedJSONValue, " ")

	if len(parts) != 2 || parts[1] != "weeks" {
		return ErrInvalidTimelineFormat
	}

	i, err := strconv.ParseInt(parts[0], 10, 32)
	if err != nil {
		return ErrInvalidTimelineFormat
	}

	*t = Timeline(i)

	return nil
}

func (t *Timeline) Scan(value interface{}) error {
	if value == nil {
		return nil
	}

	switch v := value.(type) {
	case int32:
		*t = Timeline(v)
	case int64:
		*t = Timeline(v)
	default:
		return fmt.Errorf("cannot scan %T into Timeline", value)
	}

	return nil
}
