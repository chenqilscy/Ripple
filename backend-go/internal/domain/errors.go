// Package domain 定义 Ripple 的领域模型。
//
// 本包不依赖任何持久化、传输或框架。
// 所有不变量（状态机合法性、权限层级）在此实现。
//
// 约束规约：docs/system-design/系统约束规约.md §2.2 §3.2
package domain

import "errors"

// ErrInvalidStateTransition 状态机不允许的迁移。
var ErrInvalidStateTransition = errors.New("invalid node state transition")

// ErrPermissionDenied 调用者权限不足。
var ErrPermissionDenied = errors.New("permission denied")

// ErrNotFound 资源不存在。
var ErrNotFound = errors.New("not found")

// ErrAlreadyExists 唯一性冲突。
var ErrAlreadyExists = errors.New("already exists")

// ErrInvalidInput 输入校验失败。
var ErrInvalidInput = errors.New("invalid input")

// ErrQuotaExceeded 资源配额不足。
var ErrQuotaExceeded = errors.New("quota exceeded")

// ErrConflict 并发写冲突（乐观锁 CAS 校验失败）。
var ErrConflict = errors.New("conflict")
