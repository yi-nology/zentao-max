// Package update 实现软件版本检查与更新包下载。
//
// 设计（2026-10-29 与用户对齐）：
//   - 启动注册检查：服务启动后带 {app, version, os, arch} 请求检查地址，
//     自建更新服务可据此统计活跃实例并返回最新版本信息；
//   - 定期复查（默认 24h）；检查失败静默（离线/VPN 场景不打扰）；
//   - 双数据源：check_url 配置自建服务（POST JSON）；留空回退 GitHub
//     Releases（GET latest，query 附带 version/os/arch 供透传统计）；
//   - 更新动作 = 下载更新包到本地（不做自替换/自动重启——单二进制自更新
//     在 Windows 上无法替换运行中的 exe，跨平台风险高），下载完成后
//     提示用户替换二进制并重启。
//   - 隐私：仅上报 app 名、版本、GOOS/GOARCH，无任何用户/业务数据；
//     update.enabled=false 可整体关闭。
package update

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/yi-nology/zentao-max/backend/internal/config"
	"github.com/yi-nology/zentao-max/backend/pkg/logger"
)

// Info 版本检查结果的快照（/api/update/status 返回形态）。
type Info struct {
	Current      string     `json:"current"`
	Latest       string     `json:"latest"`
	HasUpdate    bool       `json:"hasUpdate"`
	DownloadURL  string     `json:"downloadUrl"`
	Notes        string     `json:"notes"`
	Source       string     `json:"source"` // github | custom
	CheckedAt    *time.Time `json:"checkedAt"`
	LastError    string     `json:"lastError"`
	Checking     bool       `json:"checking"`
	OS           string     `json:"os"`
	Arch         string     `json:"arch"`
	DownloadState string    `json:"downloadState"` // none|downloading|done|failed
	DownloadPath string     `json:"downloadPath"`
	DownloadErr  string     `json:"downloadError"`
	// 下载进度（downloading 态下前端展示进度条）
	DownloadTotal   int64   `json:"downloadTotal"`
	Downloaded      int64   `json:"downloaded"`
	DownloadPercent float64 `json:"downloadPercent"`
	// 应用更新（解压提取新二进制并替换自身）
	Applied          bool   `json:"applied"`
	NewBinaryPath    string `json:"newBinaryPath"`
	RestartSupported bool   `json:"restartSupported"` // unix=true（syscall.Exec 自重启）；windows=false
}

// Service 版本检查/下载服务。并发安全；AutoCheck 由 HTTPApp.Start 拉起。
type Service struct {
	cfg config.UpdateConfig
	hc  *http.Client

	mu       sync.Mutex
	info     Info
	checking bool
}

// New 创建服务。currentVersion 来自 ldflags 注入的 internal/version。
func New(cfg config.UpdateConfig, currentVersion string) *Service {
	if cfg.IntervalHours <= 0 {
		cfg.IntervalHours = 24
	}
	if cfg.DownloadDir == "" {
		cfg.DownloadDir = "./data/updates"
	}
	return &Service{
		cfg: cfg,
		hc:  &http.Client{Timeout: 20 * time.Second},
		info: Info{
			Current: currentVersion,
			OS:      runtime.GOOS,
			Arch:    runtime.GOARCH,
		},
	}
}

// Status 返回当前快照。
func (s *Service) Status() Info {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.info
}

// CheckNow 立即执行一次版本检查（设置页手动触发/启动/定期共用）。
func (s *Service) CheckNow() Info {
	s.mu.Lock()
	if s.checking {
		s.mu.Unlock()
		return s.info
	}
	s.checking = true
	s.mu.Unlock()

	info := s.doCheck()
	s.mu.Lock()
	info.Checking = false
	s.info = info
	s.mu.Unlock()
	return info
}

// StartAutoCheck 启动注册检查 + 定期复查。ctx 取消即停止（进程级生命周期）。
func (s *Service) StartAutoCheck(ctx context.Context) {
	if !s.cfg.Enabled {
		return
	}
	go func() {
		time.Sleep(5 * time.Second) // 等服务与网络就绪
		s.CheckNow()
		interval := time.Duration(s.cfg.IntervalHours) * time.Hour
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				s.CheckNow()
			}
		}
	}()
}

func (s *Service) doCheck() Info {
	info := s.Status()
	info.CheckedAt = ptrTime(time.Now())
	info.LastError = ""

	latest, dl, notes, source, err := s.queryLatest()
	if err != nil {
		info.LastError = err.Error()
		return info
	}
	info.Latest = latest
	info.DownloadURL = dl
	info.Notes = notes
	info.Source = source
	info.HasUpdate = isNewer(latest, info.Current)
	return info
}

// queryLatest 双数据源查询。
func (s *Service) queryLatest() (latest, downloadURL, notes, source string, err error) {
	if s.cfg.CheckURL != "" {
		return s.queryCustom()
	}
	if s.cfg.GitHubRepo == "" {
		return "", "", "", "", fmt.Errorf("未配置 update.check_url 且 update.github_repo 为空")
	}
	return s.queryGitHub()
}

// queryGitHub GitHub Releases 模式：GET /repos/{repo}/releases/latest。
func (s *Service) queryGitHub() (string, string, string, string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest?version=%s&os=%s&arch=%s",
		s.cfg.GitHubRepo, s.info.Current, runtime.GOOS, runtime.GOARCH)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return "", "", "", "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := s.hc.Do(req)
	if err != nil {
		return "", "", "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", "", "", fmt.Errorf("GitHub API HTTP %d", resp.StatusCode)
	}
	var body struct {
		TagName string `json:"tag_name"`
		Name    string `json:"name"`
		Body    string `json:"body"`
		HTMLURL string `json:"html_url"`
		Assets  []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", "", "", "", err
	}
	dl := body.HTMLURL
	// 选取匹配当前平台地更新包资产（zentao-max-<os>-<arch>.*）
	want := fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH)
	for _, a := range body.Assets {
		if strings.Contains(a.Name, want) {
			dl = a.BrowserDownloadURL
			break
		}
	}
	notes := body.Body
	if len(notes) > 2000 {
		notes = notes[:2000]
	}
	return body.TagName, dl, notes, "github", nil
}

// queryCustom 自建更新服务模式：POST JSON 注册/检查（约定响应
// {latest_version, download_url, notes}）。
func (s *Service) queryCustom() (string, string, string, string, error) {
	payload := map[string]interface{}{
		"app":     "zentao-max",
		"version": s.info.Current,
		"os":      runtime.GOOS,
		"arch":    runtime.GOARCH,
	}
	buf, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, s.cfg.CheckURL, strings.NewReader(string(buf)))
	if err != nil {
		return "", "", "", "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.hc.Do(req)
	if err != nil {
		return "", "", "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", "", "", fmt.Errorf("更新服务 HTTP %d", resp.StatusCode)
	}
	var body struct {
		LatestVersion string `json:"latest_version"`
		DownloadURL   string `json:"download_url"`
		Notes         string `json:"notes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", "", "", "", err
	}
	if body.LatestVersion == "" {
		return "", "", "", "", fmt.Errorf("更新服务响应缺少 latest_version")
	}
	return body.LatestVersion, body.DownloadURL, body.Notes, "custom", nil
}

// Download 下载更新包到 download_dir。异步执行，进度经 Status() 轮询。
func (s *Service) Download() error {
	s.mu.Lock()
	if s.info.DownloadState == "downloading" {
		s.mu.Unlock()
		return fmt.Errorf("已有下载任务进行中")
	}
	if s.info.DownloadURL == "" {
		s.mu.Unlock()
		return fmt.Errorf("无可用下载地址（请先检查更新）")
	}
	s.info.DownloadState = "downloading"
	s.info.DownloadErr = ""
	dlURL := s.info.DownloadURL
	latest := s.info.Latest
	s.mu.Unlock()

	go func() {
		path, err := s.doDownload(dlURL, latest)
		s.mu.Lock()
		defer s.mu.Unlock()
		if err != nil {
			s.info.DownloadState = "failed"
			s.info.DownloadErr = err.Error()
			logger.Warn("更新包下载失败", "error", err)
			return
		}
		s.info.DownloadState = "done"
		s.info.DownloadPath = path
		logger.Info("更新包下载完成", "path", path)
	}()
	return nil
}

func (s *Service) doDownload(rawURL, version string) (string, error) {
	if err := os.MkdirAll(s.cfg.DownloadDir, 0755); err != nil {
		return "", err
	}
	// 下载用独立长超时 client——检查用的 20s client 对几 MB 的更新包不够
	dl := &http.Client{Timeout: 15 * time.Minute}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := dl.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("下载 HTTP %d", resp.StatusCode)
	}

	total := resp.ContentLength
	setProgress := func(n int64) {
		s.mu.Lock()
		s.info.Downloaded = n
		s.info.DownloadTotal = total
		if total > 0 {
			s.info.DownloadPercent = float64(n) / float64(total) * 100
		}
		s.mu.Unlock()
	}
	var lastEmit time.Time
	counting := &progressReader{r: resp.Body, total: total, fn: func(n int64) {
		if time.Since(lastEmit) > 400*time.Millisecond || n == total {
			lastEmit = time.Now()
			setProgress(n)
		}
	}}

	ext := ".bin"
	switch {
	case strings.Contains(rawURL, ".tar.gz"):
		ext = ".tar.gz"
	case strings.Contains(rawURL, ".zip"):
		ext = ".zip"
	}
	v := strings.TrimPrefix(version, "v")
	name := fmt.Sprintf("zentao-max-%s-%s-%s%s", v, runtime.GOOS, runtime.GOARCH, ext)
	dst := filepath.Join(s.cfg.DownloadDir, name)
	tmp := dst + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return "", err
	}
	n, err := io.Copy(f, counting)
	closeErr := f.Close()
	setProgress(n)
	if err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return "", closeErr
	}
	if n == 0 {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("下载内容为空")
	}
	if err := os.Rename(tmp, dst); err != nil {
		return "", err
	}
	return dst, nil
}

// progressReader 计数字节并回调（用于下载进度上报）。
type progressReader struct {
	r     io.Reader
	total int64
	n     int64
	fn    func(n int64)
}

func (p *progressReader) Read(buf []byte) (int, error) {
	n, err := p.r.Read(buf)
	p.n += int64(n)
	if p.fn != nil {
		p.fn(p.n)
	}
	return n, err
}

// isNewer 语义化版本比较（忽略 v 前缀/预发布后缀）；任一侧解析失败时
// 退化为"版本号不同即提示"（自建服务的版本号格式不受控）。
func isNewer(latest, current string) bool {
	lRaw := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(latest), "v"))
	cRaw := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(current), "v"))
	if lRaw == "" || cRaw == "" {
		return lRaw != "" && lRaw != cRaw
	}
	if hasNonNumeric(lRaw) || hasNonNumeric(cRaw) {
		return lRaw != cRaw
	}
	return compareSemver(normalizeVer(lRaw), normalizeVer(cRaw)) > 0
}

// hasNonNumeric 版本号含非数字段（如 "dev"、"2026.10.beta"）时不可靠比较。
func hasNonNumeric(v string) bool {
	for _, part := range strings.Split(v, ".") {
		if part == "" {
			return true
		}
		for _, ch := range part {
			if ch < '0' || ch > '9' {
				return true
			}
		}
	}
	return false
}

func normalizeVer(v string) [3]int {
	var out [3]int
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if idx := strings.IndexAny(v, "-+"); idx >= 0 {
		v = v[:idx]
	}
	parts := strings.Split(v, ".")
	for i := 0; i < 3 && i < len(parts); i++ {
		n := 0
		for _, ch := range parts[i] {
			if ch < '0' || ch > '9' {
				break
			}
			n = n*10 + int(ch-'0')
		}
		out[i] = n
	}
	return out
}

func compareSemver(a, b [3]int) int {
	for i := 0; i < 3; i++ {
		if a[i] != b[i] {
			if a[i] > b[i] {
				return 1
			}
			return -1
		}
	}
	return 0
}

func ptrTime(t time.Time) *time.Time { return &t }

// Apply 从已下载的更新包中提取新二进制并替换当前运行的可执行文件
//（unix 原子 rename；返回 restartRequired=true 表示需重启生效）。
// Windows 上无法替换运行中的 exe，返回手动操作指引。
func (s *Service) Apply() (applied bool, newBinaryPath string, err error) {
	s.mu.Lock()
	path := s.info.DownloadPath
	s.mu.Unlock()
	if path == "" {
		return false, "", fmt.Errorf("尚未下载更新包（请先下载）")
	}

	// 当前可执行文件名（zentao-max / zentao-max.exe / go-run 临时名）
	execPath, err := os.Executable()
	if err != nil {
		return false, "", err
	}
	wantName := strings.TrimSuffix(filepath.Base(execPath), ".exe")

	// 从压缩包提取二进制到临时文件
	tmpBin := path + ".new"
	if err := extractBinary(path, wantName, tmpBin); err != nil {
		return false, "", fmt.Errorf("从更新包提取二进制失败: %w", err)
	}
	if err := os.Chmod(tmpBin, 0755); err != nil {
		return false, "", err
	}

	if runtime.GOOS == "windows" {
		// Windows 无法覆盖运行中的 exe：新二进制留在下载目录，用户手动替换
		s.mu.Lock()
		s.info.Applied = true
		s.info.NewBinaryPath = tmpBin
		s.mu.Unlock()
		return true, tmpBin, nil
	}

	// unix：rename 原子替换运行中的可执行文件（旧 inode 由进程继续持有，安全）
	if err := os.Rename(tmpBin, execPath); err != nil {
		_ = os.Remove(tmpBin)
		return false, "", fmt.Errorf("替换二进制失败: %w", err)
	}
	s.mu.Lock()
	s.info.Applied = true
	s.info.NewBinaryPath = execPath
	s.mu.Unlock()
	return true, execPath, nil
}

// extractBinary 从 .tar.gz / .zip 更新包中提取名为 wantName（或带 .exe 后缀）
// 的可执行文件条目，写入 dst。
func extractBinary(archivePath, wantName, dst string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	match := func(name string) bool {
		base := filepath.Base(name)
		if base == wantName || base == wantName+".exe" {
			return true
		}
		// 归档内可能带版本号（zentao-max-0.4.1），按前缀匹配
		return strings.HasPrefix(base, "zentao-max") && base != "" &&
			!strings.Contains(base, ".sha256")
	}

	lower := strings.ToLower(archivePath)
	switch {
	case strings.HasSuffix(lower, ".zip"):
		zr, err := zip.OpenReader(archivePath)
		if err != nil {
			return fmt.Errorf("打开 zip 失败: %w", err)
		}
		defer zr.Close()
		for _, zf := range zr.File {
			if zf.FileInfo().IsDir() || !match(zf.Name) {
				continue
			}
			rc, err := zf.Open()
			if err != nil {
				return err
			}
			defer rc.Close()
			out, err := os.Create(dst)
			if err != nil {
				return err
			}
			defer out.Close()
			_, err = io.Copy(out, rc)
			return err
		}
		return fmt.Errorf("更新包内未找到 %s", wantName)
	default: // .tar.gz（及其余按 tar.gz 处理）
		gz, err := gzip.NewReader(f)
		if err != nil {
			return fmt.Errorf("打开 tar.gz 失败: %w", err)
		}
		defer gz.Close()
		tr := tar.NewReader(gz)
		for {
			hdr, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
			}
			if hdr.FileInfo().IsDir() || !match(hdr.Name) {
				continue
			}
			out, err := os.Create(dst)
			if err != nil {
				return err
			}
			defer out.Close()
			_, err = io.Copy(out, tr)
			return err
		}
		return fmt.Errorf("更新包内未找到 %s", wantName)
	}
}

// RestartSelf 以新二进制重启应用（unix：syscall.Exec 原位替换进程映像，
// 监听 fd 带 CLOEXEC 会自动关闭并由新进程重新绑定）。Windows 不支持。
func (s *Service) RestartSelf() error {
	if runtime.GOOS == "windows" {
		return fmt.Errorf("Windows 不支持自动重启，请手动停止应用后启动")
	}
	s.mu.Lock()
	bin := s.info.NewBinaryPath
	s.mu.Unlock()
	if bin == "" {
		return fmt.Errorf("尚未应用更新")
	}
	// 短延迟让 HTTP 响应先送达客户端
	go func() {
		time.Sleep(800 * time.Millisecond)
		logger.Info("应用更新完成，正在重启进程", "binary", bin)
		if err := execSelf(bin, os.Args, os.Environ()); err != nil {
			logger.Error("重启进程失败（请手动重启）", "error", err)
		}
	}()
	return nil
}
