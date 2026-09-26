package errors

type ErrorCode string

const (
	CodeNotFound                ErrorCode = "NOT_FOUND"
	CodeInvalidInput            ErrorCode = "INVALID_INPUT"
	CodeFutureDate              ErrorCode = "FUTURE_DATE"
	CodeAlreadyIngested         ErrorCode = "ALREADY_INGESTED"
	CodeBeforeInitialDate       ErrorCode = "BEFORE_INITIAL_DATE"
	CodeInvalidMeasuringPointID ErrorCode = "INVALID_MEASURING_POINT_ID"
	CodeConsultingLastState     ErrorCode = "CONSULTING_LAST_STATE"
	CodeGettingIngestedData     ErrorCode = "GETTING_INGESTED_DATA"
)
