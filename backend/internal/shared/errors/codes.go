package errors

type ErrorCode string

const (
	CodeNotFound                  ErrorCode = "NOT_FOUND"
	CodeInvalidInput              ErrorCode = "INVALID_INPUT"
	CodeConflict                  ErrorCode = "CONFLICT"
	CodeFutureDate                ErrorCode = "FUTURE_DATE"
	CodeAlreadyIngested           ErrorCode = "ALREADY_INGESTED"
	CodeBeforeInitialDate         ErrorCode = "BEFORE_INITIAL_DATE"
	CodeInvalidMeasuringPointID   ErrorCode = "INVALID_MEASURING_POINT_ID"
	CodeConsultingLastState       ErrorCode = "CONSULTING_LAST_STATE"
	CodeGettingIngestedData       ErrorCode = "GETTING_INGESTED_DATA"
	CodeConsultingDevices         ErrorCode = "CONSULTING_DEVICES"
	CodeConsultingMeasurements    ErrorCode = "CONSULTING_MEASUREMENTS"
	CodeConsultingBaseline        ErrorCode = "CONSULTING_BASELINE"
	CodePersistingBaseline        ErrorCode = "PERSISTING_BASELINE"
	CodeConsultingEvaluations     ErrorCode = "CONSULTING_EVALUATIONS"
	CodePersistingEvaluations     ErrorCode = "PERSISTING_EVALUATIONS"
	CodeConsultingThresholds      ErrorCode = "CONSULTING_THRESHOLDS"
	CodePersistingThresholds      ErrorCode = "PERSISTING_THRESHOLDS"
	CodeConsultingRuleEvaluations ErrorCode = "CONSULTING_RULE_EVALUATIONS"
	CodePersistingRuleEvaluations ErrorCode = "PERSISTING_RULE_EVALUATIONS"
	CodeConsultingAlerts          ErrorCode = "CONSULTING_ALERTS"
	CodePersistingAlerts          ErrorCode = "PERSISTING_ALERTS"
)
