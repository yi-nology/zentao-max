// Package auth 的 SSO 登录客户端：统一安全认证平台（地址由 auth.sso 配置提供）。
// 流程移植自 ky-work-mcp/internal/auth/sso.go（与 sso_login.sh 同源实测）：
//
//  1. 用登录页硬编码的 1024-bit RSA 公钥加密密码（PKCS#1 v1.5 → base64）
//  2. POST /mobiles     → 返回账号绑定的脱敏手机号（186****1498）
//  3. POST /verifyCode  → 服务端向该手机发送短信验证码
//  4. POST /sso/doLogin → 携带短信验证码登录，Set-Cookie 返回 cvaToken
//
// 平台侧拿到 doLogin 成功后自签发平台会话令牌（复用 Manager.signToken），
// 不直接使用 cvaToken。
package auth

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// SSOPublicKeyPEM 是 SSO 登录页 login.js 硬编码的 1024-bit RSA 公钥。
const SSOPublicKeyPEM = `-----BEGIN PUBLIC KEY-----
MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQCy/xHzxbaMhUbVH85Yb7G6j3p0
v6K1hunYvcL1w47b4ipzeTyui/0Cv2qJ2Z1N3HT6R5dU0nRiHs0PbcFxKGmk6YB
80KemoRxSfgSn2hRMvirXOqfvGS2kf39B2u9LK6npqt0rslJKl0KxcTtf+hlyTjE
CSSs5TThqqd8vRi05WwIDAQAB
-----END PUBLIC KEY-----`

const (
	// Referer 必须是 SSO 认证页，服务端可能校验；认证页地址由部署配置
	// （auth.sso.auth_page_url）提供，未配置时回退到 {base_url}/sso/auth
	ssoAuthPageFallback = "/sso/auth"
	ssoUserAgent        = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Safari/537.36"
)

// SSOClient 统一认证平台登录客户端（用户名+密码+短信验证码协议）。可安全并发使用。
// 平台地址不内置默认值，全部来自部署配置（auth.sso.base_url / auth_page_url）。
type SSOClient struct {
	BaseURL     string
	AuthPageURL string
	http        *http.Client
}

// NewSSOClient 构造 SSO 客户端。baseURL 为空表示未配置（调用方应先判断再启用）。
func NewSSOClient(baseURL, authPageURL string) *SSOClient {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	authPageURL = strings.TrimSpace(authPageURL)
	if authPageURL == "" && baseURL != "" {
		authPageURL = baseURL + ssoAuthPageFallback
	}
	return &SSOClient{
		BaseURL:     baseURL,
		AuthPageURL: authPageURL,
		http:        &http.Client{Timeout: 30 * time.Second},
	}
}

// SendSMSResult 是 SendCode 的返回：脱敏手机号用于页面提示"验证码已发送至 186****1498"。
type SendSMSResult struct {
	Mobile string `json:"mobile"`
}

// SendCode 执行步骤 1-3：RSA 加密密码 → 拿手机号 → 触发短信发送。
func (s *SSOClient) SendCode(ctx context.Context, username, password string) (*SendSMSResult, error) {
	passEnc, err := rsaEncryptPassword(password)
	if err != nil {
		return nil, err
	}
	mobile, err := s.postMobiles(ctx, username, passEnc)
	if err != nil {
		return nil, err
	}
	respBody, err := s.postJSON(ctx, "/verifyCode", map[string]any{
		"username": username,
		"password": passEnc,
		"mobile":   mobile,
	})
	if err != nil {
		return nil, err
	}
	if !strings.Contains(respBody, `"code":0`) {
		return nil, fmt.Errorf("验证码发送失败: %s", truncateString(respBody, 150))
	}
	return &SendSMSResult{Mobile: mobile}, nil
}

// DoSSOLogin 执行步骤 4：携带短信验证码完成登录。成功返回 SSO 账号名。
func (s *SSOClient) DoSSOLogin(ctx context.Context, username, password, smsCode string) (string, error) {
	passEnc, err := rsaEncryptPassword(password)
	if err != nil {
		return "", err
	}
	// doLogin 需要与 /verifyCode 一致的手机号，重新获取
	mobile, err := s.postMobiles(ctx, username, passEnc)
	if err != nil {
		return "", err
	}
	respBody, err := s.postJSON(ctx, "/sso/doLogin", map[string]any{
		"uuid":       randomHexUUID(),
		"username":   username,
		"password":   passEnc,
		"mobile":     mobile,
		"captcha":    smsCode,
		"remenberme": false,
	})
	if err != nil {
		return "", err
	}
	if !strings.Contains(respBody, `"code":0`) {
		// 提取服务端 msg 给用户看（验证码错误/过期等）
		var resp struct {
			Code int    `json:"code"`
			Msg  string `json:"msg"`
		}
		if json.Unmarshal([]byte(respBody), &resp) == nil && resp.Msg != "" {
			return "", fmt.Errorf("SSO 登录失败: %s", resp.Msg)
		}
		return "", fmt.Errorf("SSO 登录失败: %s", truncateString(respBody, 150))
	}
	return username, nil
}

// ---- HTTP 细节 ----

func (s *SSOClient) ssoHeaders() http.Header {
	return http.Header{
		"Accept":           []string{"application/json, text/javascript, */*; q=0.01"},
		"Content-Type":     []string{"application/json;charset=UTF-8"},
		"Origin":           []string{s.BaseURL},
		"Referer":          []string{s.AuthPageURL},
		"User-Agent":       []string{ssoUserAgent},
		"X-Requested-With": []string{"XMLHttpRequest"},
	}
}

func (s *SSOClient) postJSON(ctx context.Context, path string, payload map[string]any) (string, error) {
	buf, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.BaseURL+path, bytes.NewReader(buf))
	if err != nil {
		return "", err
	}
	for k, v := range s.ssoHeaders() {
		req.Header[k] = v
	}
	resp, err := s.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("SSO 请求失败 (%s): %w", path, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func (s *SSOClient) postMobiles(ctx context.Context, username, passEnc string) (string, error) {
	respBody, err := s.postJSON(ctx, "/mobiles", map[string]any{
		"username": username,
		"password": passEnc,
	})
	if err != nil {
		return "", err
	}
	// 响应：{"msg":"success","code":0,"data":["186****1498"]}
	m := regexp.MustCompile(`"data":\["([^"]+)"\]`).FindStringSubmatch(respBody)
	if len(m) < 2 {
		return "", fmt.Errorf("SSO 账号或密码错误（/mobiles 未返回手机号）")
	}
	return m[1], nil
}

// rsaEncryptPassword 用 SSO 登录页硬编码公钥加密密码（PKCS#1 v1.5 → base64）。
func rsaEncryptPassword(password string) (string, error) {
	block, _ := pem.Decode([]byte(SSOPublicKeyPEM))
	if block == nil {
		return "", errors.New("解析 SSO 公钥 PEM 失败")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("解析 SSO 公钥失败: %w", err)
	}
	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return "", errors.New("SSO 公钥不是 RSA 密钥")
	}
	ct, err := rsa.EncryptPKCS1v15(rand.Reader, rsaPub, []byte(password))
	if err != nil {
		return "", fmt.Errorf("RSA 加密失败: %w", err)
	}
	return base64.StdEncoding.EncodeToString(ct), nil
}

// randomHexUUID 返回 32 位大写十六进制随机串（对齐 sso_login.sh 的 openssl rand -hex 16）。
func randomHexUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return strings.ToUpper(hex.EncodeToString(b))
}

func truncateString(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
