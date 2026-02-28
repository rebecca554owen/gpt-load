package errors

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

// APIError 定义 API 响应的标准错误结构。
type APIError struct {
	HTTPStatus int
	Code       string
	Message    string
}

// Error implements the error interface.
func (e *APIError) Error() string {
	return e.Message
}

// ServiceError 定义服务层操作的结构化错误。
// 可以使用 errors.Is() 进行包装和比较。
type ServiceError struct {
	Err     error
	Message string
}

// Error implements the error interface.
func (e *ServiceError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return e.Err.Error()
}

// Unwrap 实现 errors.Is/As 的错误解包接口。
func (e *ServiceError) Unwrap() error {
	return e.Err
}

// 预定义的服务层错误，可以使用 errors.Is 进行比较
var (
	ErrBatchSizeExceedsLimit = errors.New("batch size exceeds the limit")
	ErrNoValidKeysFound      = errors.New("no valid keys found")
)

// 预定义的 API 错误
var (
	ErrBadRequest         = &APIError{HTTPStatus: http.StatusBadRequest, Code: "BAD_REQUEST", Message: "Invalid request parameters"}
	ErrInvalidJSON        = &APIError{HTTPStatus: http.StatusBadRequest, Code: "INVALID_JSON", Message: "Invalid JSON format"}
	ErrValidation         = &APIError{HTTPStatus: http.StatusBadRequest, Code: "VALIDATION_FAILED", Message: "Input validation failed"}
	ErrDuplicateResource  = &APIError{HTTPStatus: http.StatusConflict, Code: "DUPLICATE_RESOURCE", Message: "Resource already exists"}
	ErrResourceNotFound   = &APIError{HTTPStatus: http.StatusNotFound, Code: "NOT_FOUND", Message: "Resource not found"}
	ErrInternalServer     = &APIError{HTTPStatus: http.StatusInternalServerError, Code: "INTERNAL_SERVER_ERROR", Message: "An unexpected error occurred"}
	ErrDatabase           = &APIError{HTTPStatus: http.StatusInternalServerError, Code: "DATABASE_ERROR", Message: "Database operation failed"}
	ErrUnauthorized       = &APIError{HTTPStatus: http.StatusUnauthorized, Code: "UNAUTHORIZED", Message: "Authentication failed"}
	ErrForbidden          = &APIError{HTTPStatus: http.StatusForbidden, Code: "FORBIDDEN", Message: "You do not have permission to access this resource"}
	ErrTaskInProgress     = &APIError{HTTPStatus: http.StatusConflict, Code: "TASK_IN_PROGRESS", Message: "A task is already in progress"}
	ErrBadGateway         = &APIError{HTTPStatus: http.StatusBadGateway, Code: "BAD_GATEWAY", Message: "Upstream service error"}
	ErrNoActiveKeys       = &APIError{HTTPStatus: http.StatusServiceUnavailable, Code: "NO_ACTIVE_KEYS", Message: "No active API keys available for this group"}
	ErrMaxRetriesExceeded = &APIError{HTTPStatus: http.StatusBadGateway, Code: "MAX_RETRIES_EXCEEDED", Message: "Request failed after maximum retries"}
	ErrNoKeysAvailable    = &APIError{HTTPStatus: http.StatusServiceUnavailable, Code: "NO_KEYS_AVAILABLE", Message: "No API keys available to process the request"}
)

// NewServiceError 创建一个新的 ServiceError，包装基础错误并附带自定义消息。
func NewServiceError(baseErr error, message string) error {
	return &ServiceError{
		Err:     baseErr,
		Message: message,
	}
}

// NewServiceErrorf 创建一个新的 ServiceError，包装基础错误并附带格式化消息。
func NewServiceErrorf(baseErr error, format string, args ...any) error {
	return &ServiceError{
		Err:     baseErr,
		Message: fmt.Sprintf(format, args...),
	}
}

// NewAPIError 创建一个带有自定义消息的新 APIError。
func NewAPIError(base *APIError, message string) *APIError {
	return &APIError{
		HTTPStatus: base.HTTPStatus,
		Code:       base.Code,
		Message:    message,
	}
}

// NewAPIErrorWithUpstream 创建一个专门用于包装原始上游错误的新 APIError。
func NewAPIErrorWithUpstream(statusCode int, code string, upstreamMessage string) *APIError {
	return &APIError{
		HTTPStatus: statusCode,
		Code:       code,
		Message:    upstreamMessage,
	}
}

// ParseDBError 智能地将 GORM 错误转换为标准 APIError。
func ParseDBError(err error) *APIError {
	if err == nil {
		return nil
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrResourceNotFound
	}

	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == "23505" {
		return ErrDuplicateResource
	}

	if mysqlErr, ok := errors.AsType[*mysql.MySQLError](err); ok && mysqlErr.Number == 1062 {
		return ErrDuplicateResource
	}

	// SQLite 通用检查
	if strings.Contains(strings.ToLower(err.Error()), "unique constraint failed") {
		return ErrDuplicateResource
	}

	return ErrDatabase
}
