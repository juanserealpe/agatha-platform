package dtos

// Estados de la respuesta del callback (ambos responden 200).
const (
	CallbackStatusStored    = "stored"
	CallbackStatusDuplicate = "duplicate"
)

// CallbackResponse confirma a Dataflow Labs que el registro quedó persistido
// (stored) o que ya se había procesado antes (duplicate).
type CallbackResponse struct {
	Status          string `json:"status"`
	RecordID        string `json:"record_id"`
	ReceivedRecords int    `json:"received_records"`
}
