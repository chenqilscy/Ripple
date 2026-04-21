package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/chenqilscy/ripple/backend-go/internal/llm"
	"github.com/chenqilscy/ripple/backend-go/internal/platform"
	"github.com/chenqilscy/ripple/backend-go/internal/realtime"
	"github.com/chenqilscy/ripple/backend-go/internal/store"
	"github.com/rs/zerolog"
)

// CloudService 造云用例：把"创意发散"封装为异步任务。
//
// 同步路径（Generate）：
//   1. 校验输入
//   2. 写 cloud_tasks(queued)
//   3. 立即返回 task_id（不等 LLM）
//
// 异步路径（AIWeaver Worker）：
//   1. ClaimNext 拿一条 queued
//   2. 调 ZhipuAI 产 N 候选
//   3. 为每个候选建一个 MIST 节点（neo4j）
//   4. MarkDone(node_ids)
type CloudService struct {
	tasks store.CloudTaskRepository
	nodes store.NodeRepository
	lakes store.LakeRepository // 用于校验 lake 存在
}

// NewCloudService 装配。
func NewCloudService(tasks store.CloudTaskRepository, nodes store.NodeRepository, lakes store.LakeRepository) *CloudService {
	return &CloudService{tasks: tasks, nodes: nodes, lakes: lakes}
}

// CreateCloudInput 造云入参。
type CreateCloudInput struct {
	LakeID   string // 可空：节点保持 MIST 不归湖
	Prompt   string
	N        int
	NodeType domain.NodeType
}

// Generate 入队任务并立即返回。
func (s *CloudService) Generate(ctx context.Context, owner *domain.User, in CreateCloudInput) (*domain.CloudTask, error) {
	prompt := strings.TrimSpace(in.Prompt)
	if prompt == "" {
		return nil, fmt.Errorf("%w: prompt required", domain.ErrInvalidInput)
	}
	if len([]rune(prompt)) > 1000 {
		return nil, fmt.Errorf("%w: prompt too long (>1000 chars)", domain.ErrInvalidInput)
	}
	if in.N <= 0 {
		in.N = 5
	}
	if in.N > domain.MaxCloudN {
		return nil, fmt.Errorf("%w: n must be <= %d", domain.ErrInvalidInput, domain.MaxCloudN)
	}
	if in.NodeType == "" {
		in.NodeType = domain.NodeTypeText
	}
	if !in.NodeType.IsValid() {
		return nil, fmt.Errorf("%w: invalid node type", domain.ErrInvalidInput)
	}
	// lake_id 给了就要存在
	if in.LakeID != "" {
		if _, err := s.lakes.GetByID(ctx, in.LakeID); err != nil {
			return nil, err
		}
	}
	t := &domain.CloudTask{
		ID:        platform.NewID(),
		OwnerID:   owner.ID,
		LakeID:    in.LakeID,
		Prompt:    prompt,
		N:         in.N,
		NodeType:  in.NodeType,
		Status:    domain.CloudStatusQueued,
		CreatedAt: time.Now().UTC(),
	}
	if err := s.tasks.Create(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

// GetTask 查询任务。owner 校验：仅 owner 能看自己的任务（最小权限）。
func (s *CloudService) GetTask(ctx context.Context, actor *domain.User, id string) (*domain.CloudTask, error) {
	t, err := s.tasks.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if t.OwnerID != actor.ID {
		return nil, domain.ErrPermissionDenied
	}
	return t, nil
}

// ListMyTasks 我的最近任务。
func (s *CloudService) ListMyTasks(ctx context.Context, actor *domain.User, limit int) ([]domain.CloudTask, error) {
	return s.tasks.ListByOwner(ctx, actor.ID, limit)
}

// ============ AI Weaver ============

// AIWeaver 造云任务工人池。
//
// 启动时调用 RecoverRunning 把卡住的 running 还原为 queued
// （处理上次进程崩溃留下的脏状态）。
type AIWeaver struct {
	tasks   store.CloudTaskRepository
	nodes   store.NodeRepository
	llm     llm.Client
	broker  realtime.Broker // 可空
	log     zerolog.Logger
	workers int
	pollGap time.Duration
}

// NewAIWeaver 装配。workers <= 0 时默认 3。
func NewAIWeaver(
	tasks store.CloudTaskRepository,
	nodes store.NodeRepository,
	client llm.Client,
	broker realtime.Broker,
	log zerolog.Logger,
	workers int,
) *AIWeaver {
	if workers <= 0 {
		workers = 3
	}
	return &AIWeaver{
		tasks:   tasks,
		nodes:   nodes,
		llm:     client,
		broker:  broker,
		log:     log,
		workers: workers,
		pollGap: 1 * time.Second,
	}
}

// Run 阻塞直到 ctx 取消。启动 N 个 worker 并发拉任务。
func (w *AIWeaver) Run(ctx context.Context) {
	if recovered, err := w.tasks.RecoverRunning(ctx); err == nil && recovered > 0 {
		w.log.Warn().Int64("count", recovered).Msg("ai weaver recovered running tasks back to queued")
	}
	w.log.Info().Int("workers", w.workers).Msg("ai weaver started")
	for i := 0; i < w.workers; i++ {
		go w.workerLoop(ctx, i)
	}
	<-ctx.Done()
	w.log.Info().Msg("ai weaver stopped")
}

func (w *AIWeaver) workerLoop(ctx context.Context, idx int) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		t, err := w.tasks.ClaimNext(ctx)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				select {
				case <-ctx.Done():
					return
				case <-time.After(w.pollGap):
				}
				continue
			}
			w.log.Error().Err(err).Int("worker", idx).Msg("claim failed")
			time.Sleep(w.pollGap)
			continue
		}
		w.process(ctx, t, idx)
	}
}

func (w *AIWeaver) process(ctx context.Context, t *domain.CloudTask, idx int) {
	w.log.Info().Str("task", t.ID).Int("worker", idx).Int("n", t.N).Msg("ai weaver processing")

	llmCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	candidates, err := w.llm.Generate(llmCtx, t.Prompt, t.N)
	if err != nil {
		w.log.Error().Err(err).Str("task", t.ID).Msg("llm failed")
		_ = w.tasks.MarkFailed(ctx, t.ID, err.Error())
		return
	}
	if len(candidates) == 0 {
		_ = w.tasks.MarkFailed(ctx, t.ID, "llm returned empty")
		return
	}
	now := time.Now().UTC()
	mistTTL := now.Add(7 * 24 * time.Hour)
	nodeIDs := make([]string, 0, len(candidates))
	for _, content := range candidates {
		n := &domain.Node{
			ID:        platform.NewID(),
			LakeID:    t.LakeID, // 可空：MIST 节点不归湖
			OwnerID:   t.OwnerID,
			Content:   content,
			Type:      t.NodeType,
			State:     domain.StateMist,
			CreatedAt: now,
			UpdatedAt: now,
			TTLAt:     &mistTTL,
		}
		if err := w.nodes.Create(ctx, n); err != nil {
			w.log.Error().Err(err).Str("task", t.ID).Msg("node create failed")
			continue
		}
		nodeIDs = append(nodeIDs, n.ID)
		// 广播：让在线用户看到 MIST 浮云
		if w.broker != nil && t.LakeID != "" {
			_ = w.broker.Publish(ctx, realtime.LakeTopic(t.LakeID), realtime.Message{
				Type: "node.mist",
				Payload: map[string]any{
					"node_id":  n.ID,
					"task_id":  t.ID,
					"content":  content,
					"owner_id": n.OwnerID,
				},
			})
		}
	}
	if err := w.tasks.MarkDone(ctx, t.ID, nodeIDs); err != nil {
		w.log.Error().Err(err).Str("task", t.ID).Msg("mark done failed")
		return
	}
	w.log.Info().Str("task", t.ID).Int("nodes", len(nodeIDs)).Msg("ai weaver done")
}
