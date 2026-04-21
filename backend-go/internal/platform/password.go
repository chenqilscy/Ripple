// Package platform 提供横切性的低层工具：密码、JWT、ID、日志。
package platform

import (
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// MaxBcryptInputBytes bcrypt 的字节数上限（72 字节）。
const MaxBcryptInputBytes = 72

// ErrPasswordTooShort 密码强度不足。
var ErrPasswordTooShort = errors.New("password too short")

// HashPassword 安全哈希密码。
//
// 关键约束（审查报告 L2-01）：bcrypt 仅消耗前 72 字节，
// 必须按 UTF-8 字符边界截断而非裸字节切片，否则可能拆出非法字节序列。
func HashPassword(password string) (string, error) {
	if len(password) < 8 {
		return "", ErrPasswordTooShort
	}
	truncated := truncateUTF8Bytes(password, MaxBcryptInputBytes)
	h, err := bcrypt.GenerateFromPassword([]byte(truncated), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("bcrypt hash: %w", err)
	}
	return string(h), nil
}

// VerifyPassword 验证密码。
func VerifyPassword(hash, password string) bool {
	truncated := truncateUTF8Bytes(password, MaxBcryptInputBytes)
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(truncated)) == nil
}

// truncateUTF8Bytes 按字符边界把字符串截断到 maxBytes 之内。
// 不会切碎 UTF-8 多字节字符，超出部分整字符丢弃。
func truncateUTF8Bytes(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	// 逐 rune 累加字节数，保留完整字符
	cut := 0
	for i := range s {
		if i > maxBytes {
			break
		}
		cut = i
	}
	return s[:cut]
}
