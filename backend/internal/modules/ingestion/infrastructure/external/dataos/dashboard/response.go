package dashboard

import "encoding/json"

type RenderResponse struct {
	ConfigID   int64          `json:"config_id"`
	Page       string         `json:"page"`
	Period     ResponsePeriod `json:"period"`
	Timezone   string         `json:"timezone"`
	Components []Component    `json:"components"`
}

type ResponsePeriod struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type Component struct {
	ID    string          `json:"id"`
	Type  string          `json:"type"`
	Title string          `json:"title"`
	Data  json.RawMessage `json:"data"`
	Error *string         `json:"error"`
}

type ChartData struct {
	Series []Series `json:"series"`
}

type Series struct {
	Name       string      `json:"name"`
	DataPoints []DataPoint `json:"data_points"`
	Unit       string      `json:"unit"`
}

type DataPoint struct {
	Timestamp string  `json:"timestamp"`
	Value     float64 `json:"value"`
}
