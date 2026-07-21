package errors

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
)

// TestAppError_Error 测试错误消息格式
func TestAppError_Error(t *testing.T) {
	tests := []struct {
		name     string
		appErr   *AppError
		expected string
	}{
		{
			name: "无原始错误",
			appErr: &AppError{
				Code:    CodeBadRequest,
				Message: "请求错误",
			},
			expected: "[40000] 请求错误",
		},
		{
			name: "有原始错误",
			appErr: &AppError{
				Code:    CodeInternalError,
				Message: "服务器错误",
				Err:     fmt.Errorf("数据库连接失败"),
			},
			expected: "[50000] 服务器错误: 数据库连接失败",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.appErr.Error()
			if result != tt.expected {
				t.Errorf("期望=%s, 实际=%s", tt.expected, result)
			}
		})
	}
}

// TestAppError_Unwrap 测试错误解包
func TestAppError_Unwrap(t *testing.T) {
	originalErr := fmt.Errorf("原始错误")
	appErr := &AppError{
		Code:    CodeInternalError,
		Message: "服务器错误",
		Err:     originalErr,
	}

	unwrapped := appErr.Unwrap()
	if unwrapped != originalErr {
		t.Errorf("期望解包后的错误为原始错误")
	}
}

// TestAppError_HTTPStatus 测试HTTP状态码映射
func TestAppError_HTTPStatus(t *testing.T) {
	tests := []struct {
		name     string
		code     ErrorCode
		expected int
	}{
		{"成功", CodeSuccess, http.StatusOK},
		{"请求错误", CodeBadRequest, http.StatusBadRequest},
		{"参数无效", CodeInvalidParam, http.StatusBadRequest},
		{"未授权", CodeUnauthorized, http.StatusUnauthorized},
		{"禁止访问", CodeForbidden, http.StatusForbidden},
		{"资源不存在", CodeNotFound, http.StatusNotFound},
		{"方法不允许", CodeMethodNotAllowed, http.StatusMethodNotAllowed},
		{"内部错误", CodeInternalError, http.StatusInternalServerError},
		{"数据库错误", CodeDatabaseError, http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			appErr := &AppError{Code: tt.code}
			status := appErr.HTTPStatus()
			if status != tt.expected {
				t.Errorf("错误码 %d: 期望状态码=%d, 实际状态码=%d", tt.code, tt.expected, status)
			}
		})
	}
}

// TestNew 测试创建新错误
func TestNew(t *testing.T) {
	appErr := New(CodeBadRequest, "请求错误")

	if appErr.Code != CodeBadRequest {
		t.Errorf("期望 Code=%d, 实际 Code=%d", CodeBadRequest, appErr.Code)
	}
	if appErr.Message != "请求错误" {
		t.Errorf("期望 Message=请求错误, 实际 Message=%s", appErr.Message)
	}
	if appErr.Err != nil {
		t.Errorf("期望 Err=nil, 实际 Err=%v", appErr.Err)
	}
}

// TestWrap 测试包装错误
func TestWrap(t *testing.T) {
	originalErr := fmt.Errorf("原始错误")
	appErr := Wrap(CodeInternalError, "服务器错误", originalErr)

	if appErr.Code != CodeInternalError {
		t.Errorf("期望 Code=%d, 实际 Code=%d", CodeInternalError, appErr.Code)
	}
	if appErr.Err != originalErr {
		t.Errorf("期望 Err 为原始错误")
	}
	if appErr.Details != originalErr.Error() {
		t.Errorf("期望 Details=%s, 实际 Details=%s", originalErr.Error(), appErr.Details)
	}
}

// TestWrapWithDetails 测试包装错误并添加详细信息
func TestWrapWithDetails(t *testing.T) {
	originalErr := fmt.Errorf("原始错误")
	appErr := WrapWithDetails(CodeInternalError, "服务器错误", originalErr, "详细错误信息")

	if appErr.Details != "详细错误信息" {
		t.Errorf("期望 Details=详细错误信息, 实际 Details=%s", appErr.Details)
	}
}

// TestNewBadRequest 测试创建请求错误
func TestNewBadRequest(t *testing.T) {
	appErr := NewBadRequest("请求格式错误")

	if appErr.Code != CodeBadRequest {
		t.Errorf("期望 Code=%d, 实际 Code=%d", CodeBadRequest, appErr.Code)
	}
	if appErr.Message != "请求格式错误" {
		t.Errorf("期望 Message=请求格式错误, 实际 Message=%s", appErr.Message)
	}
}

// TestNewInvalidParam 测试创建参数无效错误
func TestNewInvalidParam(t *testing.T) {
	appErr := NewInvalidParam("username")

	if appErr.Code != CodeInvalidParam {
		t.Errorf("期望 Code=%d, 实际 Code=%d", CodeInvalidParam, appErr.Code)
	}
	if appErr.Message != "参数 username 无效" {
		t.Errorf("期望 Message='参数 username 无效', 实际 Message=%s", appErr.Message)
	}
}

// TestNewMissingParam 测试创建缺少参数错误
func TestNewMissingParam(t *testing.T) {
	appErr := NewMissingParam("productId")

	if appErr.Code != CodeMissingParam {
		t.Errorf("期望 Code=%d, 实际 Code=%d", CodeMissingParam, appErr.Code)
	}
	if appErr.Message != "缺少必要参数: productId" {
		t.Errorf("期望 Message='缺少必要参数: productId', 实际 Message=%s", appErr.Message)
	}
}

// TestNewInvalidID 测试创建ID无效错误
func TestNewInvalidID(t *testing.T) {
	appErr := NewInvalidID("产品ID")

	if appErr.Code != CodeInvalidID {
		t.Errorf("期望 Code=%d, 实际 Code=%d", CodeInvalidID, appErr.Code)
	}
	if appErr.Message != "无效的产品ID" {
		t.Errorf("期望 Message='无效的产品ID', 实际 Message=%s", appErr.Message)
	}
}

// TestNewNotFound 测试创建资源不存在错误
func TestNewNotFound(t *testing.T) {
	appErr := NewNotFound("产品")

	if appErr.Code != CodeNotFound {
		t.Errorf("期望 Code=%d, 实际 Code=%d", CodeNotFound, appErr.Code)
	}
	if appErr.Message != "产品不存在" {
		t.Errorf("期望 Message='产品不存在', 实际 Message=%s", appErr.Message)
	}
}

// TestNewInternalError 测试创建内部错误
func TestNewInternalError(t *testing.T) {
	originalErr := fmt.Errorf("数据库错误")
	appErr := NewInternalError("服务器内部错误", originalErr)

	if appErr.Code != CodeInternalError {
		t.Errorf("期望 Code=%d, 实际 Code=%d", CodeInternalError, appErr.Code)
	}
	if appErr.Err != originalErr {
		t.Errorf("期望 Err 为原始错误")
	}
}

// TestExternalError 测试创建外部服务错误
func TestExternalError(t *testing.T) {
	originalErr := fmt.Errorf("连接超时")
	appErr := ExternalError("禅道", originalErr)

	if appErr.Code != CodeExternalError {
		t.Errorf("期望 Code=%d, 实际 Code=%d", CodeExternalError, appErr.Code)
	}
	if appErr.Message != "禅道服务调用失败" {
		t.Errorf("期望 Message='禅道服务调用失败', 实际 Message=%s", appErr.Message)
	}
}

// TestDatabaseError 测试创建数据库错误
func TestDatabaseError(t *testing.T) {
	originalErr := fmt.Errorf("查询失败")
	appErr := DatabaseError("查询用户", originalErr)

	if appErr.Code != CodeDatabaseError {
		t.Errorf("期望 Code=%d, 实际 Code=%d", CodeDatabaseError, appErr.Code)
	}
	if appErr.Message != "数据库操作失败: 查询用户" {
		t.Errorf("期望 Message='数据库操作失败: 查询用户', 实际 Message=%s", appErr.Message)
	}
}

// TestIsAppError 测试判断是否为应用错误
func TestIsAppError(t *testing.T) {
	appErr := NewBadRequest("请求错误")
	normalErr := fmt.Errorf("普通错误")

	if !IsAppError(appErr) {
		t.Errorf("期望 IsAppError(appErr) = true")
	}
	if IsAppError(normalErr) {
		t.Errorf("期望 IsAppError(normalErr) = false")
	}
}

// TestGetAppError 测试获取应用错误
func TestGetAppError(t *testing.T) {
	t.Run("已经是AppError", func(t *testing.T) {
		appErr := NewBadRequest("请求错误")
		result := GetAppError(appErr)

		if result != appErr {
			t.Errorf("期望返回相同的 AppError")
		}
	})

	t.Run("普通错误转换", func(t *testing.T) {
		normalErr := fmt.Errorf("普通错误")
		result := GetAppError(normalErr)

		if result.Code != CodeInternalError {
			t.Errorf("期望 Code=%d, 实际 Code=%d", CodeInternalError, result.Code)
		}
		if result.Err != normalErr {
			t.Errorf("期望 Err 为原始错误")
		}
	})
}

// TestErrorChain 测试错误链
func TestErrorChain(t *testing.T) {
	originalErr := fmt.Errorf("原始错误")
	wrappedErr := Wrap(CodeInternalError, "包装错误", originalErr)

	// 使用 errors.Is 检查错误链
	if !errors.Is(wrappedErr, originalErr) {
		t.Errorf("期望错误链中包含原始错误")
	}

	// 使用 errors.Unwrap 解包错误
	unwrapped := errors.Unwrap(wrappedErr)
	if unwrapped != originalErr {
		t.Errorf("期望解包后得到原始错误")
	}
}
