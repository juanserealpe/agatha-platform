package mapping

import "time"

const dataOSTimeFormat = "2006-01-02T15:04:05"

type TimestampParser struct{}

func NewTimestampParser() *TimestampParser {
	return &TimestampParser{}
}

func (p *TimestampParser) Parse(
	value string,
) (time.Time, error) {
	return time.Parse(
		dataOSTimeFormat,
		value,
	)
}
