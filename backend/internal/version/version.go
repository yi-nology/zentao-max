package version

// 这些变量通过 -ldflags 在构建时注入
var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
	GoVersion = "unknown"
)

// Info 返回完整的版本信息
func Info() map[string]string {
	return map[string]string{
		"version":   Version,
		"buildTime": BuildTime,
		"gitCommit": GitCommit,
		"goVersion": GoVersion,
	}
}
