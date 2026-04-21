package store

import (
	"context"
	"sync"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/llm"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PGCallRecorder 把 llm.CallRecord 异步写入 llm_calls 表。
//
// 用 buffered channel + 单 worker，发送方非阻塞（channel 满则丢弃，避免拖慢业务）。
// Close 时排空残余记录。
type PGCallRecorder struct {
	pg     *pgxpool.Pool
	ch     chan llm.CallRecord
	done   chan struct{}

	mu     sync.RWMutex
	closed bool
}

// NewPGCallRecorder 启动后台 worker。bufferSize 建议 256~1024。
func NewPGCallRecorder(pg *pgxpool.Pool, bufferSize int) *PGCallRecorder {
	if bufferSize <= 0 {
		bufferSize = 512
	}
	r := &PGCallRecorder{
		pg:   pg,
		ch:   make(chan llm.CallRecord, bufferSize),
		done: make(chan struct{}),
	}
	go r.loop()
	return r
}

// Record 实现 llm.CallRecorder。非阻塞，channel 满则丢弃。
// Close 后调用安全（直接丢弃，不 panic）。
func (r *PGCallRecorder) Record(rec llm.CallRecord) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.closed {
		return
	}
	select {
	case r.ch <- rec:
	default:
		// 丢弃：审计写入不能反压业务
	}
}

// Close 优雅关闭：先标记关闭（防新 Record 写入），再关 channel，等 worker 排空。
// 多次 Close 安全。
func (r *PGCallRecorder) Close() {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return
	}
	r.closed = true
	r.mu.Unlock()
	close(r.ch)
	<-r.done
}

func (r *PGCallRecorder) loop() {
	defer close(r.done)
	const sql = `
INSERT INTO llm_calls (provider, modality, prompt_hash, candidates_n, cost_tokens, latency_ms, status, error_message)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
`
	for rec := range r.ch {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		_, _ = r.pg.Exec(ctx, sql,
			rec.Provider, string(rec.Modality), rec.PromptHash,
			rec.CandidatesN, rec.CostTokens, rec.LatencyMS,
			rec.Status, rec.ErrorMessage,
		)
		cancel()
	}
}
