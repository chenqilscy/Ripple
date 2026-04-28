package store

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/chenqilscy/ripple/backend-go/internal/llm"
	"github.com/jackc/pgx/v5/pgxpool"
)

// LLMCallsAnalyticsRepository 提供 llm_calls 表的分析查询（Phase 15-D）。
type LLMCallsAnalyticsRepository interface {
	// SumByProvider 返回指定 org 在 since 之后各 provider 的调用汇总。
	SumByProvider(ctx context.Context, orgID string, since time.Time) ([]domain.ProviderUsage, error)
	// SumByDay 返回按天的调用量汇总。
	SumByDay(ctx context.Context, orgID string, since time.Time) ([]domain.DayUsage, error)
}

type llmCallsAnalyticsPG struct{ pool *pgxpool.Pool }

// NewLLMCallsAnalyticsRepository 构造。
func NewLLMCallsAnalyticsRepository(pool *pgxpool.Pool) LLMCallsAnalyticsRepository {
	return &llmCallsAnalyticsPG{pool: pool}
}

const sqlLLMCallsByProvider = `
SELECT provider,
       COUNT(*) AS calls,
       COALESCE(AVG(latency_ms)::int, 0) AS avg_duration_ms
FROM llm_calls
WHERE org_id = $1
  AND created_at >= $2
GROUP BY provider
ORDER BY calls DESC
`

func (r *llmCallsAnalyticsPG) SumByProvider(ctx context.Context, orgID string, since time.Time) ([]domain.ProviderUsage, error) {
	rows, err := r.pool.Query(ctx, sqlLLMCallsByProvider, orgID, since)
	if err != nil {
		return nil, fmt.Errorf("llm_calls sum by provider: %w", err)
	}
	defer rows.Close()

	var out []domain.ProviderUsage
	for rows.Next() {
		var p domain.ProviderUsage
		if err := rows.Scan(&p.Provider, &p.Calls, &p.AvgDurationMS); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

const sqlLLMCallsByDay = `
SELECT TO_CHAR(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD') AS day,
			 COUNT(*) AS calls,
			 COALESCE(SUM(
					 CASE provider
							 WHEN 'zhipu' THEN 0.01
							 WHEN 'deepseek' THEN 0.008
							 WHEN 'openai' THEN 0.02
							 WHEN 'volc' THEN 0.012
							 WHEN 'minimax' THEN 0.015
							 ELSE 0.01
					 END
			 ), 0) AS estimated_cost_cny
FROM llm_calls
WHERE org_id = $1
  AND created_at >= $2
GROUP BY day
ORDER BY day ASC
`

func (r *llmCallsAnalyticsPG) SumByDay(ctx context.Context, orgID string, since time.Time) ([]domain.DayUsage, error) {
	rows, err := r.pool.Query(ctx, sqlLLMCallsByDay, orgID, since)
	if err != nil {
		return nil, fmt.Errorf("llm_calls sum by day: %w", err)
	}
	defer rows.Close()

	var out []domain.DayUsage
	for rows.Next() {
		var d domain.DayUsage
		if err := rows.Scan(&d.Date, &d.Calls, &d.EstimatedCostCNY); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

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
