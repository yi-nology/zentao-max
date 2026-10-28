//go:build wireinject
// +build wireinject

package app

import (
	"log"
	"os"

	"github.com/yi-nology/zentao-max/backend/core/initialization"
	"github.com/yi-nology/zentao-max/backend/core/metrics"
	"github.com/yi-nology/zentao-max/backend/core/zentao"

	"github.com/google/wire"
)

// InitializeHTTPApp 初始化HTTP应用
// 使用wire自动生成依赖注入代码
func InitializeHTTPApp(config *AppConfig) (Application, error) {
	wire.Build(
		provideInitService,
		provideZentaoClient,
		provideConfigStore,
		provideMetrics,
		NewDependencies,
		provideHTTPApp,
	)
	return nil, nil
}

func provideInitService(config *AppConfig) *initialization.InitService {
	return initialization.NewInitService(
		config.AuthDBPath,
		config.EncryptionKey,
	)
}

func provideConfigStore(config *AppConfig) *initialization.ConfigStore {
	return initialization.NewConfigStore(config.AuthDBPath)
}

// provideMetrics 初始化全局 metrics（HTTP server 启动时已经调过 Init，但 wire 装配是
// 并行/重入安全的，重复调用返回 nil）。返回的 *Metrics 通过 SessionMetricsAdapter
// 注入 zentao.Client，让 SessionCache + SessionWorkerPool 内部回调能上报 prometheus。
func provideMetrics(config *AppConfig) *metrics.Metrics {
	_ = metrics.Init()
	return metrics.Get()
}

// provideZentaoClient 提供ZentaoClient实例
func provideZentaoClient(config *AppConfig, initService *initialization.InitService, m *metrics.Metrics) *zentao.Client {
	// 加载禅道配置（含 realm）
	zentaoServer, zentaoAccount, zentaoPassword, realm := initialization.LoadZentaoConfigWithRealm(initService)

	// 如果配置中有值，优先使用配置中的值
	if config.ZentaoServer != "" {
		zentaoServer = config.ZentaoServer
	}
	if config.ZentaoAccount != "" {
		zentaoAccount = config.ZentaoAccount
	}
	if config.ZentaoPassword != "" {
		zentaoPassword = config.ZentaoPassword
	}

	// 如果仍然没有配置，从环境变量获取（优先 ZENTAO_MAX_ 前缀）
	if zentaoServer == "" {
		zentaoServer = os.Getenv("ZENTAO_MAX_ZENTAO_SERVER")
		if zentaoServer == "" {
			zentaoServer = os.Getenv("ZENTAO_SERVER")
		}
	}
	if zentaoAccount == "" {
		zentaoAccount = os.Getenv("ZENTAO_MAX_ZENTAO_ACCOUNT")
		if zentaoAccount == "" {
			zentaoAccount = os.Getenv("ZENTAO_ACCOUNT")
		}
	}
	if zentaoPassword == "" {
		zentaoPassword = os.Getenv("ZENTAO_MAX_ZENTAO_PASSWORD")
		if zentaoPassword == "" {
			zentaoPassword = os.Getenv("ZENTAO_PASSWORD")
		}
	}
	if realm == "" {
		if r := os.Getenv("ZENTAO_MAX_REALM"); r != "" {
			realm = r
		} else if r := os.Getenv("ZENTAO_REALM"); r != "" {
			realm = r
		}
	}

	// 统一走会话模式（PHP *.json 端点）。realm 可选：非空（如 "kydc"）走受限实例 SSO；
	// 空走禅道本地（md5 混淆）。
	//
	// Session 模式专属：cache L2 持久化到 ~/.zentao-max/cache-session.db，
	// pool 用默认限流（8/s, 4 读 worker, 写串行）。两者都 nil-safe：空配置时跳过。
	cacheDBPrefix := "~/.zentao-max/cache"
	if home, err := os.UserHomeDir(); err == nil {
		cacheDBPrefix = home + "/.zentao-max/cache"
	}
	poolCfg := &zentao.SessionWorkerPoolConfig{} // 默认值
	client := zentao.NewSessionClient(zentaoServer, zentaoAccount, zentaoPassword, realm, cacheDBPrefix, poolCfg)

	// 注入 metrics 适配器（SessionCache + SessionWorkerPool 内部回调到 prometheus）。
	if client != nil && m != nil {
		client.AttachMetrics(&metrics.SessionMetricsAdapter{M: m})
	}

	// zentaosid 注入（受限实例 SSO 限流 bypass）：
	// 优先读 ~/.zentao-max/zentaosid.json（用户浏览器粘贴的 zentaosid），
	// 有就调 Client.SetZentaoSID 注入 cookie jar，后续请求直接用现成 cookie，
	// 不再触发 server 端 login()，不消耗受限实例 SSO "密码尝试次数太多" 配额。
	// 没文件就 fallback 走正常 login 流程（第一次需要 <user-account> 密码）。
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

// provideHTTPApp 提供HTTPApp并绑定到Application接口
func provideHTTPApp(config *AppConfig, deps *Dependencies) Application {
	return NewHTTPApp(config, deps)
}
