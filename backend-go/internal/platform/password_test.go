package platform

import (
	"strings"
	"testing"
	"time"
)

func TestHashAndVerify_Basic(t *testing.T) {
	h, err := HashPassword("hello-world-12345")
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPassword(h, "hello-world-12345") {
		t.Fatal("verify failed")
	}
	if VerifyPassword(h, "wrong-password") {
		t.Fatal("verify should have failed")
	}
}

func TestHashPassword_TooShort(t *testing.T) {
	if _, err := HashPassword("short"); err != ErrPasswordTooShort {
		t.Fatalf("expected ErrPasswordTooShort, got %v", err)
	}
}

func TestHashPassword_LongUTF8DoesNotPanic(t *testing.T) {
	// 60 个中文字符 ≈ 180 字节 > 72，必须按字符边界截断且不 panic。
	long := strings.Repeat("青萍", 30)
	h, err := HashPassword(long)
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPassword(h, long) {
		t.Fatal("verify same long pwd failed")
	}
}

func TestTruncateUTF8Bytes_CharBoundary(t *testing.T) {
	// 单个汉字 3 字节，cap=5 应只保留 1 个汉字（3 字节），不切碎。
	out := truncateUTF8Bytes("青萍", 5)
	if out != "青" {
		t.Fatalf("got %q, want 青", out)
	}
}

func TestJWTRoundTrip(t *testing.T) {
	s := NewJWTSigner("dev-secret-1234567890", time.Hour)
	tok, err := s.Sign("user-1", "alice@x.io")
	if err != nil {
		t.Fatal(err)
	}
	c, err := s.Parse(tok)
	if err != nil {
		t.Fatal(err)
	}
	if c.UserID != "user-1" || c.Email != "alice@x.io" {
		t.Fatalf("claims mismatch: %+v", c)
	}
}

func TestJWTParse_BadSecret(t *testing.T) {
	s1 := NewJWTSigner("secret-A", time.Hour)
	s2 := NewJWTSigner("secret-B", time.Hour)
	tok, _ := s1.Sign("u", "e@e")
	if _, err := s2.Parse(tok); err != ErrInvalidToken {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}

func TestJWTParse_Expired(t *testing.T) {
	s := NewJWTSigner("secret", -time.Hour)
	tok, _ := s.Sign("u", "e@e")
	if _, err := s.Parse(tok); err != ErrInvalidToken {
		t.Fatalf("expected ErrInvalidToken (expired), got %v", err)
	}
}
