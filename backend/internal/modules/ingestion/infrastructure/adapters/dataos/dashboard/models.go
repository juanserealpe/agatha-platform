package dashboard

type RenderRequest struct {
	Period           Period `json:"period"`
	Timezone         string `json:"timezone"`
	MeasuringPointID int64  `json:"measuring_point_id"`
}

type Period struct {
	Type  string `json:"type"`
	Start string `json:"start"`
	End   string `json:"end"`
}
