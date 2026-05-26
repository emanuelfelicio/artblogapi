package response

type ErrorCode string

const (
	ParseCode          ErrorCode = "PARSE_ERROR"
	ValidationCode     ErrorCode = "VALIDATION_ERROR"
	ConflictCode       ErrorCode = "CONFLICT_ERROR"
	UnauthorizedCode   ErrorCode = "UNAUTHORIZED"
	ForbiddenCode      ErrorCode = "FORBIDDEN"
	InternalServerCode ErrorCode = "INTERNAL_SERVER_ERROR"
)
