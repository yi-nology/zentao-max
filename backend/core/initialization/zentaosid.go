package initialization

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ZentaoSIDFile 是浏览器 zentaosid 持久化文件的结构。
//
// 存储位置：~/.zentao-max/zentaosid.json（明文，因为只是 cookie 不是密码）。
//
// 用途：把浏览器登录 your-zentao-server.com 拿到的 zentaosid cookie 写入这个
// 文件 → server 启动时 wire.go 读这个文件调 Client.SetZentaoSID 注入 →
// 后续所有禅道请求直接用这个 zentaosid，不再触发 server 端 login()，从而
// 不消耗受限实例 SSO "密码尝试次数太多" 配额。
type ZentaoSIDFile struct {
	ZentaoSID string    `json:"zentaosid"`
	UpdatedAt time.Time `json:"updatedAt"`
	Note      string    `json:"note,omitempty"`
}

const (
	zentaoSIDFileName = "zentaosid.json"
	zentaoSIDFileMode  = 0600 // cookie 不是密码但仍限制权限
)

var zentaoSIDFileMu sync.Mutex

// ZentaoSIDFilePath 拼接 zentaosid.json 的完整路径。
// 优先 $ZENTAO_MAX_ZENTAOSID_PATH（覆盖），否则 ~/.zentao-max/zentaosid.json。
func ZentaoSIDFilePath() (string, error) {
	if p := os.Getenv("ZENTAO_MAX_ZENTAOSID_PATH"); p != "" {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get user home: %w", err)
	}
	return filepath.Join(home, ".zentao-max", zentaoSIDFileName), nil
}

// SaveZentaoSID 持久化 zentaosid 到磁盘（0600 权限，目录不存在时创建）。
// 并发安全（全局锁）。
func SaveZentaoSID(sid, note string) error {
	zentaoSIDFileMu.Lock()
	defer zentaoSIDFileMu.Unlock()

	path, err := ZentaoSIDFilePath()
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}

	payload := ZentaoSIDFile{
		ZentaoSID: sid,
		UpdatedAt: time.Now().UTC(),
		Note:      note,
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, zentaoSIDFileMode); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// LoadZentaoSID 从磁盘读 zentaosid。文件不存在 / 解析失败 / sid 为空都返 err。
func LoadZentaoSID() (string, error) {
	zentaoSIDFileMu.Lock()
	defer zentaoSIDFileMu.Unlock()

	path, err := ZentaoSIDFilePath()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var payload ZentaoSIDFile
	if err := json.Unmarshal(data, &payload); err != nil {
		return "", fmt.Errorf("parse %s: %w", path, err)
	}
	if payload.ZentaoSID == "" {
		return "", fmt.Errorf("%s: zentaosid 字段为空", path)
	}
	return payload.ZentaoSID, nil
}

// DeleteZentaoSID 删 zentaosid.json（清掉注入的 zentaosid，下次启动走正常 login 路径）。
func DeleteZentaoSID() error {
	zentaoSIDFileMu.Lock()
	defer zentaoSIDFileMu.Unlock()

	path, err := ZentaoSIDFilePath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
