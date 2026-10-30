package initialization

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// AuthConfig 认证配置结构
type AuthConfig struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Domain   string `json:"domain"`
	// Realm 认证域。非空（如 "kydc"）表示使用会话模式登录，
	// 数据访问走禅道 PHP *.json 端点；空表示默认 Token 模式（REST API）。
	// 适用于禁用 REST API 的禅道实例（如受限实例 your-zentao-server.com）。
	Realm string `json:"realm,omitempty"`
}

// EncryptedAuthConfig 加密的认证配置结构
type EncryptedAuthConfig struct {
	Salt          string `json:"salt"`
	Iv            string `json:"iv"`
	EncryptedData string `json:"encrypted_data"`
}

// InitService 初始化服务
type InitService struct {
	dbPath        string
	encryptionKey string
	// legacyKey 旧版机器派生密钥，仅用于回退解密 encryption.key 方案启用前
	// 写入的 auth.db；新写入一律用 encryptionKey（显式配置 > 密钥文件）。
	legacyKey string
}

// keyFileName 加密密钥文件名（与 auth.db 同目录，随其一起备份/搬迁）。
const keyFileName = "encryption.key"

// deriveLegacyMachineKey 旧版机器派生密钥。
//
// ⚠️ 该方案已被密钥文件取代：macOS 的 kern hostname（os.Hostname 的返回值）会随
// 网络/VPN 环境漂移（如 "Mac" ↔ "mofeiyideMacBook-Air"），hostname 一变密钥即变，
// 旧 auth.db 立刻解密失败（invalid character ...）——这正是历史上 auth.db 反复
// 失效的根因。仅保留作回退解密用，不再用于新写入。
func deriveLegacyMachineKey() string {
	hostname, _ := os.Hostname()
	homeDir, _ := os.UserHomeDir()
	h := sha256.Sum256([]byte(hostname + ":" + homeDir + ":zentao-max"))
	return hex.EncodeToString(h[:])
}

// keyFilePath 密钥文件与 auth.db 同目录。
func (s *InitService) keyFilePath() string {
	return filepath.Join(filepath.Dir(s.dbPath), keyFileName)
}

// loadOrCreateKeyFile 加载或首次生成托管密钥文件（0600）。
// 随机 32 字节 hex，生成后固定不变；读写失败时回退旧机器派生（与历史行为一致）。
func (s *InitService) loadOrCreateKeyFile() string {
	path := s.keyFilePath()
	if data, err := os.ReadFile(path); err == nil {
		if key := strings.TrimSpace(string(data)); key != "" {
			// 权限自愈：历史版本或手工创建可能留下宽权限，收紧到 0600
			if info, serr := os.Stat(path); serr == nil && info.Mode().Perm() != 0600 {
				if cerr := os.Chmod(path, 0600); cerr != nil {
					log.Printf("WARNING: failed to tighten key file permissions: %v", cerr)
				}
			}
			return key
		}
	}
	keyBytes := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, keyBytes); err != nil {
		log.Println("WARNING: failed to generate key file, falling back to machine-derived key:", err)
		return deriveLegacyMachineKey()
	}
	key := hex.EncodeToString(keyBytes)
	// auth.db 同目录（如 ./data/）在全新安装时还不存在；不建目录会导致密钥文件
	// 写失败并回退到 hostname 派生密钥，重新引入机器名漂移即解密失败的老问题。
	var werr error
	if werr = os.MkdirAll(filepath.Dir(path), 0o755); werr == nil {
		werr = os.WriteFile(path, []byte(key+"\n"), 0600)
	}
	if werr != nil {
		log.Println("WARNING: failed to write key file, falling back to machine-derived key:", werr)
		return deriveLegacyMachineKey()
	}
	log.Printf("Generated managed encryption key file: %s", path)
	return key
}

// NewInitService 创建初始化服务实例
func NewInitService(dbPath, encryptionKey string) *InitService {
	if dbPath == "" {
		// 无显式路径时按统一规则解析（env → 已存在候选 → 规范位置），
		// 见 auth_path.go；打包应用遗留的 ~/.zentao-max 位置也在候选之列。
		dbPath = ResolveAuthDBPath("")
	}
	if encryptionKey == "" {
		encryptionKey = os.Getenv("ZENTAO_ENCRYPTION_KEY")
		if encryptionKey == "" {
			encryptionKey = os.Getenv("ZENTAO_MAX_SECURITY_ENCRYPTION_KEY")
		}
	}

	s := &InitService{dbPath: dbPath, encryptionKey: encryptionKey}
	if encryptionKey == "" {
		// 无显式密钥：使用托管密钥文件（首次生成后稳定，不受网络环境影响），
		// 并保留旧机器派生密钥用于回退解密历史 auth.db。
		log.Println("WARNING: ZENTAO_ENCRYPTION_KEY not set; using managed key file next to auth.db (production should set the env var explicitly)")
		s.encryptionKey = s.loadOrCreateKeyFile()
		s.legacyKey = deriveLegacyMachineKey()
	}
	return s
}

// InitStatus 初始化状态结构
type InitStatus struct {
	IsFirstStart bool   `json:"isFirstStart"`
	HasConfig    bool   `json:"hasConfig"`
	Message      string `json:"message"`
}

// IsFirstStart 检测是否为首次启动
func (s *InitService) IsFirstStart() (bool, error) {
	// 检查数据库文件是否存在
	fileInfo, err := os.Stat(s.dbPath)
	if err != nil {
		if os.IsNotExist(err) {
			// 数据库文件不存在，判定为首次启动
			return true, nil
		}
		return false, err
	}

	if fileInfo.Size() == 0 {
		// 数据库文件为空，判定为首次启动
		return true, nil
	}

	// 数据库文件存在且不为空，判定为非首次启动
	return false, nil
}

// GetInitStatus 获取详细的初始化状态
func (s *InitService) GetInitStatus() *InitStatus {
	status := &InitStatus{
		IsFirstStart: true,
		HasConfig:    false,
		Message:      "首次启动，请上传配置文件",
	}

	fileInfo, err := os.Stat(s.dbPath)
	if err != nil {
		if os.IsNotExist(err) {
			return status
		}
		status.Message = fmt.Sprintf("检查配置文件状态失败: %v", err)
		return status
	}

	if fileInfo.Size() == 0 {
		return status
	}

	status.HasConfig = true
	status.IsFirstStart = false

	fileData, err := os.ReadFile(s.dbPath)
	if err != nil {
		status.Message = fmt.Sprintf("读取配置文件失败: %v", err)
		return status
	}

	var encryptedConfig EncryptedAuthConfig
	if err := json.Unmarshal(fileData, &encryptedConfig); err != nil {
		status.Message = fmt.Sprintf("配置文件格式无效: %v", err)
		return status
	}

	_, _, err = s.decrypt(encryptedConfig.EncryptedData, encryptedConfig.Salt, encryptedConfig.Iv)
	if err != nil {
		status.Message = fmt.Sprintf("配置文件验证失败: %v", err)
		return status
	}

	status.Message = "系统已初始化，配置有效"
	return status
}

// StoreAuthConfig 存储认证配置到数据库
func (s *InitService) StoreAuthConfig(fileData []byte) error {

	// 确保目录存在
	dbDir := filepath.Dir(s.dbPath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return fmt.Errorf("failed to create db directory: %w", err)
	}

	// 原子写：先写临时文件再 rename，进程中断不会留下半写的 auth.db
	tmp := s.dbPath + ".tmp"
	if err := os.WriteFile(tmp, fileData, 0600); err != nil {
		return fmt.Errorf("failed to write auth config to tmp file: %w", err)
	}
	if err := os.Rename(tmp, s.dbPath); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("failed to rename auth config into place: %w", err)
	}

	return nil
}

// StoreAuthConfigFromStruct 加密一个 AuthConfig 后落盘。供登录表单端点使用：
// 用户直接提交账号密码时（而非上传加密文件），用本地 encryptionKey 做 AES 加密
// 后写入 auth.db，保证密码不以明文形式落盘。salt 用随机 hex，IV 由 Encrypt 生成。
func (s *InitService) StoreAuthConfigFromStruct(cfg *AuthConfig) error {
	if cfg == nil {
		return errors.New("auth config is nil")
	}
	// 随机 salt（16 字节 hex）。
	saltBytes := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, saltBytes); err != nil {
		return fmt.Errorf("failed to generate salt: %w", err)
	}
	salt := hex.EncodeToString(saltBytes)

	ciphertextB64, ivHex, err := s.Encrypt(cfg, salt)
	if err != nil {
		return fmt.Errorf("failed to encrypt auth config: %w", err)
	}

	encrypted := EncryptedAuthConfig{
		Salt:          salt,
		Iv:            ivHex,
		EncryptedData: ciphertextB64,
	}
	data, err := json.Marshal(encrypted)
	if err != nil {
		return fmt.Errorf("failed to marshal encrypted config: %w", err)
	}
	return s.StoreAuthConfig(data)
}

// LoadAuthConfig 从数据库加载认证配置
func (s *InitService) LoadAuthConfig() (*AuthConfig, []byte, error) {
	// 读取数据库文件（存储的是加密的JSON文件原文）
	fileData, err := os.ReadFile(s.dbPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read auth config from db: %w", err)
	}

	// 解析加密配置
	var encryptedConfig EncryptedAuthConfig
	if err := json.Unmarshal(fileData, &encryptedConfig); err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal encrypted config: %w", err)
	}

	// 解密数据
	authConfig, legacyUsed, err := s.decrypt(encryptedConfig.EncryptedData, encryptedConfig.Salt, encryptedConfig.Iv)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decrypt auth config: %w", err)
	}
	if legacyUsed {
		// 自动迁移：用主密钥（托管密钥文件）重新加密落盘，下次读取不再依赖
		// 会随网络环境漂移的旧机器派生密钥。失败仅告警，不影响本次返回——
		// 旧文件下次读取仍可回退解密。
		if merr := s.StoreAuthConfigFromStruct(authConfig); merr != nil {
			log.Printf("WARNING: auth.db legacy-key migration failed (will retry on next load): %v", merr)
		} else {
			log.Println("auth.db migrated to managed key file")
		}
	}

	return authConfig, fileData, nil
}

// decrypt 解密数据。先试主密钥（显式配置 / 托管密钥文件），失败且存在旧机器
// 派生密钥时回退再试一次——兼容密钥文件方案启用前写入的 auth.db。
// legacyUsed 为 true 表示本次是回退解密命中，调用方应择机迁移到主密钥。
func (s *InitService) decrypt(encryptedData, salt, iv string) (*AuthConfig, bool, error) {
	config, err := s.decryptWithKey(s.encryptionKey, encryptedData, salt, iv)
	if err == nil {
		return config, false, nil
	}
	if s.legacyKey != "" {
		if cfg, legacyErr := s.decryptWithKey(s.legacyKey, encryptedData, salt, iv); legacyErr == nil {
			log.Println("auth.db decrypted with legacy machine-derived key; migrating to managed key file")
			return cfg, true, nil
		}
	}
	return nil, false, err
}

// decryptWithKey 用指定密钥解密。
//
// quirk：key = (encryptionKey + salt)[:32] 后再整体 SHA256——encryptionKey 为
// 64 字符 hex 时截断吃满 32 字节，salt 实际不参与有效密钥（与历史 shell 脚本
// 行为保持一致的遗留兼容）。修复会破坏现有 auth.db 解密，故保留；安全等级由
// 密钥本身（256-bit 随机托管文件）保证。
func (s *InitService) decryptWithKey(key, encryptedData, salt, iv string) (*AuthConfig, error) {
	// 解码base64数据
	decodedData, err := base64.StdEncoding.DecodeString(encryptedData)
	if err != nil {
		return nil, err
	}

	// 尝试直接解析为AuthConfig（处理base64编码的情况）
	var config AuthConfig
	if err := json.Unmarshal(decodedData, &config); err == nil {
		// 直接解析成功，返回结果
		return &config, nil
	}

	// 尝试AES解密（处理真正加密的情况）
	// 检查IV是否提供
	if iv == "" {
		return nil, errors.New("IV is required for AES decryption")
	}

	// 解码IV
	ivBytes, err := hex.DecodeString(iv)
	if err != nil {
		return nil, fmt.Errorf("failed to decode IV: %w", err)
	}

	// 创建密钥（与shell脚本保持一致，使用SHA-256哈希）
	raw := []byte(key + salt)
	// 确保密钥长度为32字节（与shell脚本保持一致）
	if len(raw) > 32 {
		raw = raw[:32]
	}
	keyHash := sha256.Sum256(raw)
	keyBytes := keyHash[:]

	// 创建AES加密块
	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return nil, err
	}

	// 检查密文长度
	if len(decodedData) == 0 {
		return nil, errors.New("ciphertext is empty")
	}

	// 创建解密器
	stream := cipher.NewCFBDecrypter(block, ivBytes)

	// 解密数据
	plaintext := make([]byte, len(decodedData))
	stream.XORKeyStream(plaintext, decodedData)

	// 解析认证配置
	if err := json.Unmarshal(plaintext, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// Encrypt 加密数据（用于测试）
// 返回 (base64密文, hex编码IV, error)
func (s *InitService) Encrypt(config *AuthConfig, salt string) (string, string, error) {
	configData, err := json.Marshal(config)
	if err != nil {
		return "", "", err
	}

	key := []byte(s.encryptionKey + salt)
	if len(key) > 32 {
		key = key[:32]
	}
	keyHash := sha256.Sum256(key)
	keyBytes := keyHash[:]

	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return "", "", err
	}

	iv := make([]byte, aes.BlockSize)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return "", "", err
	}

	stream := cipher.NewCFBEncrypter(block, iv)
	ciphertext := make([]byte, len(configData))
	stream.XORKeyStream(ciphertext, configData)

	return base64.StdEncoding.EncodeToString(ciphertext), hex.EncodeToString(iv), nil
}

// LoadZentaoConfig 加载禅道配置
func LoadZentaoConfig(initService *InitService) (string, string, string) {
	server, account, password, _ := LoadZentaoConfigWithRealm(initService)
	return server, account, password
}

// LoadZentaoConfigWithRealm 加载禅道配置，额外返回 realm。
// realm 非空表示会话模式（kydc），空表示 Token 模式。
// 优先级：数据库 > 环境变量（ZENTAO_REALM / ZENTAO_MAX_REALM）。
func LoadZentaoConfigWithRealm(initService *InitService) (server, account, password, realm string) {
	log.Println("Loading zentao config...")

	authConfig, _, err := initService.LoadAuthConfig()
	if err == nil && authConfig != nil {
		log.Println("Config loaded from database successfully")
		log.Printf("Using database config: Domain=%s, Username=%s, Realm=%s", authConfig.Domain, authConfig.Username, authConfig.Realm)
		return authConfig.Domain, authConfig.Username, authConfig.Password, authConfig.Realm
	}

	log.Printf("Failed to load config from database: %v", err)
	log.Println("Falling back to environment variables")

	server = os.Getenv("ZENTAO_SERVER")
	account = os.Getenv("ZENTAO_ACCOUNT")
	password = os.Getenv("ZENTAO_PASSWORD")
	if realm = os.Getenv("ZENTAO_REALM"); realm == "" {
		realm = os.Getenv("ZENTAO_MAX_REALM")
	}

	log.Printf("Using environment variables: Domain=%s, Username=%s, Realm=%s", server, account, realm)
	return server, account, password, realm
}
