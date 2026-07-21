package errors

import (
	"fmt"
	"net/http"
)

// ErrorCode 统一的错误码定义
// 错误码规范：前两位表示HTTP状态码类别，后三位表示具体错误
type ErrorCode int

const (
	// 成功
	CodeSuccess ErrorCode = 200

	// 客户端错误 4xxxx
	CodeBadRequest       ErrorCode = 40000 // 通用请求错误
	CodeInvalidParam     ErrorCode = 40001 // 参数无效
	CodeMissingParam     ErrorCode = 40002 // 缺少必要参数
	CodeInvalidID        ErrorCode = 40003 // ID格式无效
	CodeInvalidDate      ErrorCode = 40004 // 日期格式无效
	CodeUnauthorized     ErrorCode = 40100 // 未授权
	CodeForbidden        ErrorCode = 40300 // 禁止访问
	CodeNotFound         ErrorCode = 40400 // 资源不存在
	CodeMethodNotAllowed ErrorCode = 40500 // 方法不允许

	// 服务端错误 5xxxx
	CodeInternalError ErrorCode = 50000 // 通用服务器错误
	CodeDatabaseError ErrorCode = 50001 // 数据库错误
	CodeExternalError ErrorCode = 50002 // 外部服务错误
	CodeTimeout       ErrorCode = 50003 // 超时错误
	CodeConfigError   ErrorCode = 50004 // 配置错误
	CodeNetworkError  ErrorCode = 50005 // 网络错误
	CodeUnknownError  ErrorCode = 50006 // 未知错误
)

// AppError 应用错误结构
type AppError struct {
	Code    ErrorCode // 错误码
	Message string    // 错误消息（对外展示）
	Err     error     // 原始错误（内部记录）
	Details string    // 详细错误信息（用于日志）
}

// Error 实现error接口
func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%d] %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

// Unwrap 支持错误解包
func (e *AppError) Unwrap() error {
	return e.Err
}

// HTTPStatus 返回对应的HTTP状态码
func (e *AppError) HTTPStatus() int {
	switch {
	case e.Code == CodeSuccess:
		return http.StatusOK
	case e.Code >= 20000 && e.Code < 30000:
		return http.StatusOK
	case e.Code >= 40000 && e.Code < 40100:
		return http.StatusBadRequest
	case e.Code >= 40100 && e.Code < 40200:
		return http.StatusUnauthorized
	case e.Code >= 40300 && e.Code < 40400:
		return http.StatusForbidden
	case e.Code >= 40400 && e.Code < 40500:
		return http.StatusNotFound
	case e.Code >= 40500 && e.Code < 50000:
		return http.StatusMethodNotAllowed
	case e.Code >= 50000:
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}

// New 创建新的应用错误
func New(code ErrorCode, message string) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
	}
}

// Wrap 包装错误
func Wrap(code ErrorCode, message string, err error) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Err:     err,
		Details: err.Error(),
	}
}

// WrapWithDetails 包装错误并添加详细信息
func WrapWithDetails(code ErrorCode, message string, err error, details string) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Err:     err,
		Details: details,
	}
}

// NewBadRequest 创建请求错误
func NewBadRequest(message string) *AppError {
	return New(CodeBadRequest, message)
}

// NewInvalidParam 创建参数无效错误
func NewInvalidParam(paramName string) *AppError {
	return New(CodeInvalidParam, fmt.Sprintf("参数 %s 无效", paramName))
}

// NewMissingParam 创建缺少参数错误
func NewMissingParam(paramName string) *AppError {
	return New(CodeMissingParam, fmt.Sprintf("缺少必要参数: %s", paramName))
}

// NewInvalidID 创建ID无效错误
func NewInvalidID(idName string) *AppError {
	return New(CodeInvalidID, fmt.Sprintf("无效的%s", idName))
}

// NewNotFound 创建资源不存在错误
func NewNotFound(resource string) *AppError {
	return New(CodeNotFound, fmt.Sprintf("%s不存在", resource))
}

// NewInternalError 创建内部错误
func NewInternalError(message string, err error) *AppError {
	return Wrap(CodeInternalError, message, err)
}

// ExternalError 创建外部服务错误
func ExternalError(service string, err error) *AppError {
	return Wrap(CodeExternalError, fmt.Sprintf("%s服务调用失败", service), err)
}

// DatabaseError 创建数据库错误
func DatabaseError(operation string, err error) *AppError {
	return Wrap(CodeDatabaseError, fmt.Sprintf("数据库操作失败: %s", operation), err)
}

// IsAppError 判断是否为应用错误
func IsAppError(err error) bool {
	_, ok := err.(*AppError)
	return ok
}

// GetAppError 获取应用错误，如果不是则转换为内部错误
func GetAppError(err error) *AppError {
	if appErr, ok := err.(*AppError); ok {
		return appErr
	}
	return Wrap(CodeInternalError, "服务器内部错误", err)
}
