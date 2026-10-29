package svc

import (
	"log"
	"os"

	"github.com/yi-nology/zentao-max/backend/internal/infra/zentao"
	"github.com/yi-nology/zentao-max/backend/internal/initialization"
)

// InitializeHTTPApp 装配应用：凭据解析（ldflags/config/env → auth.db 回退）
// → 禅道 client（含 zentaosid 注入）→ 依赖容器 → HTTPApp。
// 原 wire 生成层删除：装配对象仅 4 个，手写直读比生成代码更可读
// （对齐 octopus svc.InitServiceContext 手写装配模式）。
// 文件原名 wire_gen.go（wire 时代遗留），google/wire 移除后更名消除误导。
func InitializeHTTPApp(config *AppConfig) (*HTTPApp, error) {
	initService := initialization.NewInitService(config.AuthDBPath, config.EncryptionKey)
	configStore := initialization.NewConfigStore(config.AuthDBPath)
	client := provideZentaoClient(config, initService)
	deps := NewDependencies(initService, client, configStore)
	return NewHTTPApp(config, deps), nil
}

// provideZentaoClient 禅道 client 装配：凭据优先级
// ldflags/config（AppConfig）→ env（ZENTAO_SERVER/ACCOUNT/PASSWORD，
// 兼容 ZENTAO_MAX_ 前缀）→ auth.db 加密存储（LoadZentaoConfigWithRealm）。
func provideZentaoClient(config *AppConfig, initService *initialization.InitService) *zentao.Client {
	zentaoServer, zentaoAccount, zentaoPassword, realm := initialization.LoadZentaoConfigWithRealm(initService)

	if config.ZentaoServer != "" {
		zentaoServer = config.ZentaoServer
	}
	if config.ZentaoAccount != "" {
		zentaoAccount = config.ZentaoAccount
	}
	if config.ZentaoPassword != "" {
		zentaoPassword = config.ZentaoPassword
	}
	if zentaoServer == "" {
		zentaoServer = os.Getenv("ZENTAO_SERVER")
	}
	if zentaoAccount == "" {
		zentaoAccount = os.Getenv("ZENTAO_ACCOUNT")
	}
	if zentaoPassword == "" {
		zentaoPassword = os.Getenv("ZENTAO_PASSWORD")
	}
	if realm == "" {
		if r := os.Getenv("ZENTAO_MAX_REALM"); r != "" {
			realm = r
		} else if r := os.Getenv("ZENTAO_REALM"); r != "" {
			realm = r
		}
	}

	// 统一走 session 模式（PHP *.json 端点）。
	// realm 可选：非空（如 "kydc"）走受限实例 SSO；空走禅道本地（md5 混淆）。
	client := zentao.NewSessionClient(zentaoServer, zentaoAccount, zentaoPassword, realm, "", nil)

	// zentaosid 注入（受限实例 SSO 限流 bypass）：
	// 优先读 ~/.zentao-max/zentaosid.json（用户浏览器粘贴的 zentaosid），
	// 有就调 Client.SetZentaoSID 注入 cookie jar，后续请求直接用现成 cookie，
	// 不再触发 server 端 login()，不消耗受限实例 SSO "密码尝试次数太多" 配额。
	// 没文件就 fallback 走正常 login 流程（第一次需要 <user-account> 密码）。
	//
	// ⚠️ 必须在 client 创建之后、return 之前执行。
	if client != nil && zentaoServer != "" {
		if sid, err := initialization.LoadZentaoSID(); err == nil && sid != "" {
			if injErr := client.SetZentaoSID(sid); injErr != nil {
				log.Printf("zentaosid 注入失败（不影响启动，将走 login fallback）: server=%s err=%v", zentaoServer, injErr)
			} else {
				log.Printf("zentaosid 注入成功（受限实例 SSO 限流 bypass 启用）: server=%s", zentaoServer)
			}
		} else if err != nil {
			log.Printf("未找到 zentaosid.json（走正常 login）: path_err=%v", err)
		}
	}

	return client
}

// NewHTTPApp 构造 HTTPApp（原 provideHTTPApp，接口删除后直返具体类型）。
func NewHTTPApp(config *AppConfig, deps *Dependencies) *HTTPApp {
	return newHTTPApp(config, deps)
}
