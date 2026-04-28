package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/chenqilscy/ripple/backend-go/internal/llm"
	"github.com/chenqilscy/ripple/backend-go/internal/store"
	"github.com/rs/zerolog"
)

// AiJobWorker 是 AI 节点填充任务工人池（Phase 15-C）。
//
// 启动时调用 RecoverProcessing 把崩溃遗留的 processing 任务重置为 pending。
// Worker 数量默认 3，通过 RIPPLE_AI_WORKER_N 配置。
type AiJobWorker struct {
	jobs      store.AiJobRepository
	nodes     store.NodeRepository
	lakes     store.LakeRepository
	templates store.PromptTemplateRepository
	renderer  *PromptRenderer
	router    llm.Router
	log       zerolog.Logger
	workers   int
	pollGap   time.Duration
}

// NewAiJobWorker 装配。workers <= 0 时默认 3。
func NewAiJobWorker(
	jobs store.AiJobRepository,
	nodes store.NodeRepository,
	lakes store.LakeRepository,
	templates store.PromptTemplateRepository,
	router llm.Router,
	log zerolog.Logger,
	workers int,
) *AiJobWorker {
	if workers <= 0 {
		workers = 3
	}
	return &AiJobWorker{
		jobs:      jobs,
		nodes:     nodes,
		lakes:     lakes,
		templates: templates,
		renderer:  NewPromptRenderer(),
		router:    router,
		log:       log,
		workers:   workers,
		pollGap:   2 * time.Second,
	}
}

// Run 阻塞直到 ctx 取消。
func (w *AiJobWorker) Run(ctx context.Context) {
	if recovered, err := w.jobs.RecoverProcessing(ctx); err == nil && recovered > 0 {
		w.log.Warn().Int64("count", recovered).Msg("ai job worker recovered processing tasks to pending")
	}
	w.log.Info().Int("workers", w.workers).Msg("ai job worker started")
	for i := 0; i < w.workers; i++ {
		go w.workerLoop(ctx, i)
	}
	<-ctx.Done()
	w.log.Info().Msg("ai job worker stopped")
}

func (w *AiJobWorker) workerLoop(ctx context.Context, idx int) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		jobs, err := w.jobs.ListPending(ctx, w.workers)
		if err != nil {
			if !errors.Is(err, context.Canceled) {
				w.log.Error().Err(err).Int("worker", idx).Msg("ai job list pending failed")
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(w.pollGap):
			}
			continue
		}
		if len(jobs) == 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(w.pollGap):
			}
			continue
		}
		for i := range jobs {
			w.process(ctx, &jobs[i], idx)
		}
	}
}

func (w *AiJobWorker) process(ctx context.Context, job *domain.AiJob, idx int) {
	log := w.log.With().Str("job", job.ID).Str("node", job.NodeID).Int("worker", idx).Logger()
	log.Info().Msg("ai job processing started")

	// 1. 标记为 processing（进度 30%）
	if err := w.jobs.UpdateStatus(ctx, job.ID, domain.AiJobProcessing, 30, ""); err != nil {
		log.Error().Err(err).Msg("ai job mark processing failed")
		return
	}

	// 2. 获取节点内容
	node, err := w.nodes.GetByID(ctx, job.NodeID)
	if err != nil {
		log.Error().Err(err).Msg("ai job get node failed")
		_ = w.jobs.UpdateStatus(ctx, job.ID, domain.AiJobFailed, 0, err.Error())
		return
	}

	// 3. 获取 Prompt 模板
	var promptStr string
	if job.PromptTemplateID != "" {
		tmpl, err := w.templates.GetByID(ctx, job.PromptTemplateID)
		if err != nil {
			log.Error().Err(err).Msg("ai job get template failed")
			_ = w.jobs.UpdateStatus(ctx, job.ID, domain.AiJobFailed, 0, err.Error())
			return
		}
		// 4. 构建模板变量
		vars, vErr := w.buildVars(ctx, job, node)
		if vErr != nil {
			log.Warn().Err(vErr).Msg("ai job build vars partial failure (non-fatal)")
		}
		// 合并 override_vars（用户临时覆盖）
		for k, v := range job.OverrideVars {
			vars[k] = v
		}
		promptStr = w.renderer.Render(tmpl.Template, vars)
	} else {
		// 无模板时使用节点内容作 Prompt
		promptStr = StripHTML(node.Content)
	}

	// 5. 调用 LLM（进度 60%）
	if err := w.jobs.UpdateStatus(ctx, job.ID, domain.AiJobProcessing, 60, ""); err != nil {
		log.Error().Err(err).Msg("ai job update progress failed")
	}

	llmCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	cands, err := w.router.Generate(llmCtx, llm.GenerateRequest{
		Prompt:   promptStr,
		N:        1,
		Modality: llm.ModalityText,
		Hints:    llm.TextHints{Temperature: 0.7},
	})
	if err != nil {
		log.Error().Err(err).Msg("ai job llm call failed")
		_ = w.jobs.UpdateStatus(ctx, job.ID, domain.AiJobFailed, 0, err.Error())
		return
	}
	if len(cands) == 0 || cands[0].Text == "" {
		_ = w.jobs.UpdateStatus(ctx, job.ID, domain.AiJobFailed, 0, "llm returned empty response")
		return
	}

	// 6. 回写节点内容
	node.Content = cands[0].Text
	node.UpdatedAt = time.Now().UTC()
	if err := w.nodes.UpdateContent(ctx, node); err != nil {
		log.Error().Err(err).Msg("ai job node update failed")
		_ = w.jobs.UpdateStatus(ctx, job.ID, domain.AiJobFailed, 0, err.Error())
		return
	}

	// 7. 标记 done（进度 100%）
	if err := w.jobs.UpdateStatus(ctx, job.ID, domain.AiJobDone, 100, ""); err != nil {
		log.Error().Err(err).Msg("ai job mark done failed")
	}
	log.Info().Msg("ai job completed")
}

// buildVars 构建标准模板变量（{{node_content}} / {{lake_name}} / {{selected_nodes}} / {{user_name}}）。
func (w *AiJobWorker) buildVars(ctx context.Context, job *domain.AiJob, node *domain.Node) (map[string]string, error) {
	vars := map[string]string{
		"node_content": StripHTML(node.Content),
		"user_name":    job.CreatedBy, // fallback：UUID；handler 层可传 display_name（override_vars）
	}

	// {{lake_name}}
	if node.LakeID != "" {
		if lake, err := w.lakes.GetByID(ctx, node.LakeID); err == nil {
			vars["lake_name"] = lake.Name
		} else {
			vars["lake_name"] = node.LakeID
		}
	}

	// {{selected_nodes}}
	if len(job.InputNodeIDs) > 0 {
		var parts []string
		for _, nid := range job.InputNodeIDs {
			if n, err := w.nodes.GetByID(ctx, nid); err == nil {
				parts = append(parts, StripHTML(n.Content))
			}
		}
		vars["selected_nodes"] = strings.Join(parts, "\n---\n")
	}

	return vars, nil
}
