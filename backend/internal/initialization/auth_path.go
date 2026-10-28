package initialization

import (
	"os"
	"path/filepath"
)

// defaultAuthDBPath auth.db 的规范位置（相对进程工作目录）。
// 运行时产物（auth.db、cron.db）统一落在工作目录的 data/ 下。
const defaultAuthDBPath = "./data/auth.db"

// ResolveAuthDBPath 解析 auth.db 实际路径。所有入口（cmd/app / cmd/server / cmd/mcp）
// 统一走这里，避免各自硬编码不同位置导致配置"明明存在却读不到"。
//
// 优先级：
//  1. 显式传入（config 文件 auth.db_path / 入口变量，可被 ldflags 注入）
//  2. 环境变量 ZENTAO_AUTH_DB_PATH
//  3. 已存在的候选位置（首个存在的非空文件胜出，兼容历史布局）：
//     ./data/auth.db（规范）→ ./auth.db（历史）→ ~/.zentao-max/auth.db（最早）
//  4. 都没有 → 规范位置 ./data/auth.db（首次写入落这里）
func ResolveAuthDBPath(explicit string) string {
	if explicit != "" {
		return explicit
	}
	if v := os.Getenv("ZENTAO_AUTH_DB_PATH"); v != "" {
		return v
	}
	for _, p := range authDBPathCandidates() {
		if fi, err := os.Stat(p); err == nil && fi.Size() > 0 {
			return p
		}
	}
	return defaultAuthDBPath
}

func authDBPathCandidates() []string {
	candidates := []string{defaultAuthDBPath, "./auth.db"}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, ".zentao-max", "auth.db"))
	}
	return candidates
}
