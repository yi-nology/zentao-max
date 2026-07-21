// Package constants 定义应用级别的常量
// 消除代码中的魔法数字，提高代码可维护性
package constants

import "time"

// HTTP 相关常量
const (
	// HTTPServerReadTimeout HTTP服务器读取超时时间
	HTTPServerReadTimeout = 30 * time.Second

	// HTTPServerWriteTimeout HTTP服务器写入超时时间
	HTTPServerWriteTimeout = 30 * time.Second

	// HTTPServerIdleTimeout HTTP服务器空闲超时时间
	HTTPServerIdleTimeout = 60 * time.Second
)

// 分页相关常量
const (
	// DefaultPage 默认页码
	DefaultPage = 1

	// DefaultPageSize 默认每页数量
	DefaultPageSize = 20

	// MaxPageSize 最大每页数量
	MaxPageSize = 100

	// LargePageSize 大数据量查询时的页面大小
	LargePageSize = 1000
)

// 缓存相关常量
const (
	// TokenExpiryDuration Token缓存过期时间（23小时，保险起见）
	TokenExpiryDuration = 23 * time.Hour

	// TokenRefreshCheckInterval Token刷新检查间隔
	TokenRefreshCheckInterval = 2 * time.Hour

	// TokenRefreshThreshold Token刷新阈值（剩余时间小于此值时刷新）
	TokenRefreshThreshold = 2 * time.Hour

	// UsersCacheExpiry 用户列表缓存过期时间
	UsersCacheExpiry = 24 * time.Hour

	// ProductsCacheExpiry 产品列表缓存过期时间
	ProductsCacheExpiry = 24 * time.Hour
)

// API请求相关常量
const (
	// DefaultAPITimeout 默认API请求超时时间
	DefaultAPITimeout = 10 * time.Second

	// LongAPITimeout 长时间API请求超时时间（用于复杂查询）
	LongAPITimeout = 120 * time.Second

	// MaxRetryCount 最大重试次数
	MaxRetryCount = 3

	// InitialRetryDelay 初始重试延迟
	InitialRetryDelay = 1 * time.Second
)

// 并发控制相关常量
const (
	// DefaultWorkerCount 默认Worker数量
	DefaultWorkerCount = 3

	// HighConcurrencyWorkerCount 高并发Worker数量
	HighConcurrencyWorkerCount = 5

	// MaxTaskLimit 最大任务限制
	MaxTaskLimit = 500
)

// 分页查询相关常量
const (
	// DefaultQueryLimit 默认查询数量限制
	DefaultQueryLimit = 100

	// MaxQueryLimit 最大查询数量限制
	MaxQueryLimit = 1000
)

// HTTP状态码
const (
	// StatusSuccess 成功状态码
	StatusSuccess = 200

	// StatusBadRequest 错误请求
	StatusBadRequest = 400

	// StatusUnauthorized 未授权
	StatusUnauthorized = 401

	// StatusForbidden 禁止访问
	StatusForbidden = 403

	// StatusNotFound 资源不存在
	StatusNotFound = 404

	// StatusInternalError 服务器内部错误
	StatusInternalError = 500
)

// 错误码定义
const (
	// CodeSuccess 成功
	CodeSuccess = 20000

	// CodeBadRequest 通用请求错误
	CodeBadRequest = 40000

	// CodeInvalidParam 参数无效
	CodeInvalidParam = 40001

	// CodeMissingParam 缺少必要参数
	CodeMissingParam = 40002

	// CodeInvalidID ID格式无效
	CodeInvalidID = 40003

	// CodeInvalidDate 日期格式无效
	CodeInvalidDate = 40004

	// CodeUnauthorized 未授权
	CodeUnauthorized = 40100

	// CodeForbidden 禁止访问
	CodeForbidden = 40300

	// CodeNotFound 资源不存在
	CodeNotFound = 40400

	// CodeMethodNotAllowed 方法不允许
	CodeMethodNotAllowed = 40500

	// CodeInternalError 通用服务器错误
	CodeInternalError = 50000

	// CodeDatabaseError 数据库错误
	CodeDatabaseError = 50001

	// CodeExternalError 外部服务错误
	CodeExternalError = 50002

	// CodeTimeout 超时错误
	CodeTimeout = 50003

	// CodeConfigError 配置错误
	CodeConfigError = 50004

	// CodeNetworkError 网络错误
	CodeNetworkError = 50005

	// CodeUnknownError 未知错误
	CodeUnknownError = 50006
)

// 限流相关常量
const (
	// DefaultRateLimit 默认限流（每秒请求数）
	DefaultRateLimit = 100

	// DefaultBurstSize 默认突发大小
	DefaultBurstSize = 200

	// RateLimitWindow 限流窗口时间
	RateLimitWindow = 1 * time.Minute
)

// 日志相关常量
const (
	// DefaultLogLevel 默认日志级别
	DefaultLogLevel = "info"

	// LogMaxSize 日志文件最大大小（MB）
	LogMaxSize = 100

	// LogMaxBackups 日志文件最大备份数
	LogMaxBackups = 3

	// LogMaxAge 日志文件最大保存天数
	LogMaxAge = 7
)

// 应用相关常量
const (
	// AppName 应用名称
	AppName = "zentao-max"

	// DefaultPort 默认HTTP端口
	DefaultPort = 12345
)
