//go:build windows

package update

import "fmt"

// execSelf Windows 不支持 syscall.Exec——返回手动操作指引。
func execSelf(binaryPath string, args, env []string) error {
	return fmt.Errorf("Windows 不支持自动重启：请停止应用后，用 %s 手动启动", binaryPath)
}
