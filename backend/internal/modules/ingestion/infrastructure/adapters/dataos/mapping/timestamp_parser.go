package mapping

import "time"

type TimestampParser struct {
	location *time.Location
}

func NewTimestampParser(location *time.Location) *TimestampParser {
	return &TimestampParser{
		location: location,
	}
}

func (p *TimestampParser) Parse(value string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, err
	}

	return t.In(p.location), nil
}
