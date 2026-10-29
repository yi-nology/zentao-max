//go:build !windows

package update

import "syscall"

// execSelf unix：以新二进制原位替换进程映像（syscall.Exec）。
// Go 打开的监听 fd 带 CLOEXEC，exec 时自动关闭并由新进程重新绑定端口。
func execSelf(binaryPath string, args, env []string) error {
	return syscall.Exec(binaryPath, args, env)
}
