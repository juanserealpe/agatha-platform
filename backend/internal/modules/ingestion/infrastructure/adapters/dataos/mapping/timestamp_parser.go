package mapping

import "time"

const dataOSTimeFormat = "2006-01-02T15:04:05"

type TimestampParser struct {
	location *time.Location
}

func NewTimestampParser(location *time.Location) *TimestampParser {
	return &TimestampParser{
		location: location,
	}
}

func (p *TimestampParser) Parse(value string) (time.Time, error) {
	return time.ParseInLocation(
		dataOSTimeFormat,
		value,
		p.location,
	)
}
