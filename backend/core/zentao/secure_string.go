package zentao

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"sync"
)

// SecureString 安全字符串类型，用于存储敏感信息
// 使用XOR加密存储，防止内存扫描
type SecureString struct {
	obfuscated []byte // 混淆后的数据
	key        []byte // XOR密钥
	mu         sync.RWMutex
}

// NewSecureString 创建安全字符串
func NewSecureString(plaintext string) *SecureString {
	s := &SecureString{}
	s.Set(plaintext)
	return s
}

// Set 设置字符串值（加密存储）
func (s *SecureString) Set(plaintext string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 清除旧数据
	if s.obfuscated != nil {
		for i := range s.obfuscated {
			s.obfuscated[i] = 0
		}
	}
	if s.key != nil {
		for i := range s.key {
			s.key[i] = 0
		}
	}

	if plaintext == "" {
		s.obfuscated = nil
		s.key = nil
		return
	}

	// 生成随机密钥
	key := make([]byte, len(plaintext))
	if _, err := rand.Read(key); err != nil {
		// 如果随机数生成失败，使用简单的混淆
		key = []byte(hex.EncodeToString([]byte(plaintext)))[:len(plaintext)]
	}

	// XOR混淆
	obfuscated := make([]byte, len(plaintext))
	for i := 0; i < len(plaintext); i++ {
		obfuscated[i] = plaintext[i] ^ key[i]
	}

	s.obfuscated = obfuscated
	s.key = key
}

// Get 获取字符串值（解密）
func (s *SecureString) Get() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.obfuscated == nil || s.key == nil {
		return ""
	}

	// XOR解密
	plaintext := make([]byte, len(s.obfuscated))
	for i := 0; i < len(s.obfuscated); i++ {
		plaintext[i] = s.obfuscated[i] ^ s.key[i]
	}

	return string(plaintext)
}

// Clear 清除敏感数据
func (s *SecureString) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 安全清除内存
	if s.obfuscated != nil {
		for i := range s.obfuscated {
			s.obfuscated[i] = 0
		}
		s.obfuscated = nil
	}
	if s.key != nil {
		for i := range s.key {
			s.key[i] = 0
		}
		s.key = nil
	}
}

// EqualConstantTime 常量时间比较，防止时序攻击
func (s *SecureString) EqualConstantTime(other string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.obfuscated == nil || s.key == nil {
		return other == ""
	}

	// 解密当前值
	plaintext := make([]byte, len(s.obfuscated))
	for i := 0; i < len(s.obfuscated); i++ {
		plaintext[i] = s.obfuscated[i] ^ s.key[i]
	}

	// 常量时间比较
	return subtle.ConstantTimeCompare(plaintext, []byte(other)) == 1
}

// SecureBytes 安全字节数组类型
type SecureBytes struct {
	data []byte
	mu   sync.RWMutex
}

// NewSecureBytes 创建安全字节数组
func NewSecureBytes(data []byte) *SecureBytes {
	s := &SecureBytes{}
	s.Set(data)
	return s
}

// Set 设置字节数组
func (s *SecureBytes) Set(data []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 清除旧数据
	if s.data != nil {
		for i := range s.data {
			s.data[i] = 0
		}
	}

	if data == nil {
		s.data = nil
		return
	}

	// 复制数据
	s.data = make([]byte, len(data))
	copy(s.data, data)
}

// Get 获取字节数组
func (s *SecureBytes) Get() []byte {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.data == nil {
		return nil
	}

	// 返回副本，防止外部修改
	result := make([]byte, len(s.data))
	copy(result, s.data)
	return result
}

// Clear 清除数据
func (s *SecureBytes) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.data != nil {
		for i := range s.data {
			s.data[i] = 0
		}
		s.data = nil
	}
}
