package kernel

// 业务信封 code：成功为 0；失败默认等于 HTTP 状态。需要区分同一状态的不同原因时，用 status*100+n。
// 不使用与 HTTP 无关的 1001/2001 号段，避免和现有 42901 以及前端按 status/code 判断重试打架。
const (
	CodeOK = 0

	CodeInvalidArgument = 400
	CodeUnauthorized    = 401
	CodeForbidden       = 403
	CodeNotFound        = 404
	CodeConflict        = 409
	CodeTooManyRequests = 429
	CodeInternal        = 500
	CodeBadGateway      = 502
	CodeUnavailable     = 503
	CodeTimeout         = 504

	CodeQuotaExceeded          = 40301
	CodeInsufficientCredits    = 40001
	CodeStorageReadOnly        = 40302
	CodeIdempotencyConflict    = 40901
	CodeRateLimited            = 42901
	CodeUsernameChangeLimit    = 42902
	CodeCanvasResourcesMissing = 40902
)

// ErrorReason 是稳定机器可读原因，前端应判断 reason 而不是解析 msg。
type ErrorReason string

const (
	ReasonInvalidArgument                ErrorReason = "invalid_argument"
	ReasonUnauthorized                   ErrorReason = "unauthorized"
	ReasonForbidden                      ErrorReason = "forbidden"
	ReasonNotFound                       ErrorReason = "not_found"
	ReasonConflict                       ErrorReason = "conflict"
	ReasonFailedPrecondition             ErrorReason = "failed_precondition"
	ReasonQuotaExceeded                  ErrorReason = "quota_exceeded"
	ReasonInsufficientCredits            ErrorReason = "insufficient_credits"
	ReasonRateLimited                    ErrorReason = "rate_limited"
	ReasonUsernameChangeLimit            ErrorReason = "username_change_limit"
	ReasonAdminPermission                ErrorReason = "admin_permission_denied"
	ReasonUnavailable                    ErrorReason = "unavailable"
	ReasonTimeout                        ErrorReason = "timeout"
	ReasonInternal                       ErrorReason = "internal"
	ReasonBadGateway                     ErrorReason = "bad_gateway"
	ReasonUpstreamDNSFailed              ErrorReason = "upstream_dns_failed"
	ReasonProjectNameConflict            ErrorReason = "project_name_conflict"
	ReasonCanvasResourcesMissing         ErrorReason = "canvas_history_resources_missing"
	ReasonWorkbenchSubmissionConflict    ErrorReason = "workbench_submission_conflict"
	ReasonWorkbenchSubmissionMissing     ErrorReason = "workbench_submission_missing"
	ReasonWorkbenchSubmissionTaskRemoved ErrorReason = "workbench_submission_task_removed"
	ReasonWorkbenchRetryChanged          ErrorReason = "workbench_retry_changed"
	ReasonInspirationLibraryChanged      ErrorReason = "inspiration_library_changed"
	ReasonFeedbackChanged                ErrorReason = "feedback_changed"
	ReasonFeedbackSubmissionConflict     ErrorReason = "feedback_submission_conflict"
	ReasonSkillVersionConflict           ErrorReason = "skill_version_conflict"
	ReasonStorageCapacityExceeded        ErrorReason = "storage_capacity_exceeded"
	ReasonStorageReadOnly                ErrorReason = "storage_read_only"
)

func ReasonForStatus(status int) ErrorReason {
	switch status {
	case CodeInvalidArgument:
		return ReasonInvalidArgument
	case CodeUnauthorized:
		return ReasonUnauthorized
	case CodeForbidden:
		return ReasonForbidden
	case CodeNotFound:
		return ReasonNotFound
	case CodeConflict:
		return ReasonConflict
	case CodeTooManyRequests:
		return ReasonRateLimited
	case CodeBadGateway:
		return ReasonBadGateway
	case CodeUnavailable:
		return ReasonUnavailable
	case CodeTimeout:
		return ReasonTimeout
	default:
		if status >= 500 {
			return ReasonInternal
		}
		return ReasonInvalidArgument
	}
}
