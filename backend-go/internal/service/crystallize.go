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
	"github.com/chenqilscy/ripple/backend-go/internal/store"
)

// CrystallizeService M3-S2 起步：把若干 mist 节点凝结成一个 perma 节点。
//
// 当前实现（同步、最小可用）：
//  1. 校验 actor 对所有 source 节点拥有读权限（间接：所有节点必须同 lake，actor 是该 lake 成员）；
//  2. 拼接节点 content → LLM Generate → 取首个 candidate.Text 作 summary；
//  3. 写 PG perma_nodes 行（Neo4j :Perma 节点放在 S2.5 与 outbox 一并实现，本期不写 Neo4j）。
//
// 不做的：
//   - 不写 Neo4j :Perma 节点（避免 saga 风险，留待 S2.5）
//   - 不删/不改 source mist 节点（保留追溯）
//   - 不做异步 worker（先走同步 API，验证 LLM 模板与权限模型）
type CrystallizeService struct {
	perma   store.PermaNodeRepository
	nodes   store.NodeRepository
	members LakeMemberRepository // 复用 Lake 的权限校验
	router  llm.Router
}

// LakeMemberRepository 是 CrystallizeService 需要的最小权限接口。
// 由 Lake 的 LakeMembershipRepository 满足；这里独立出来便于单测。
type LakeMemberRepository interface {
	GetRole(ctx context.Context, userID, lakeID string) (domain.Role, error)
}

// NewCrystallizeService 装配。
func NewCrystallizeService(perma store.PermaNodeRepository, nodes store.NodeRepository, members LakeMemberRepository, router llm.Router) *CrystallizeService {
	return &CrystallizeService{perma: perma, nodes: nodes, members: members, router: router}
}

// CrystallizeInput 入参。
type CrystallizeInput struct {
	LakeID        string
	SourceNodeIDs []string // 至少 2 个，最多 20 个
	TitleHint     string   // 可选：用户给的标题提示
}

// Crystallize 同步执行凝结流程。
func (s *CrystallizeService) Crystallize(ctx context.Context, actor *domain.User, in CrystallizeInput) (*domain.PermaNode, error) {
	// 1. 入参校验
	if strings.TrimSpace(in.LakeID) == "" {
		return nil, fmt.Errorf("%w: lake_id required", domain.ErrInvalidInput)
	}
	if n := len(in.SourceNodeIDs); n < 2 || n > 20 {
		return nil, fmt.Errorf("%w: source_node_ids must be 2-20", domain.ErrInvalidInput)
	}

	// 2. 权限：actor 必须是该 lake 的成员，且角色 >= PASSENGER（OBSERVER 只读不可凝结）
	role, err := s.members.GetRole(ctx, actor.ID, in.LakeID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.ErrPermissionDenied
		}
		return nil, err
	}
	if !role.AtLeast(domain.RolePassenger) {
		return nil, domain.ErrPermissionDenied
	}

	// 3. 取每个 source 节点（顺序遍历；S2.5 改 GetManyByIDs 优化）
	contents := make([]string, 0, len(in.SourceNodeIDs))
	for _, id := range in.SourceNodeIDs {
		n, err := s.nodes.GetByID(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("source %s: %w", id, err)
		}
		// 跨湖凝结禁止
		if n.LakeID != "" && n.LakeID != in.LakeID {
			return nil, fmt.Errorf("%w: node %s not in lake %s", domain.ErrInvalidInput, id, in.LakeID)
		}
		contents = append(contents, fmt.Sprintf("- %s", n.Content))
	}

	// 4. LLM 凝结 prompt（最小可用模板）
	titleHint := strings.TrimSpace(in.TitleHint)
	if titleHint == "" {
		titleHint = "（自动生成）"
	}
	prompt := fmt.Sprintf(
		"你是凝结助手。请把以下 %d 条想法凝结成一段简洁的总结，输出 80~200 字：\n\n%s\n\n标题提示：%s",
		len(contents), strings.Join(contents, "\n"), titleHint,
	)
	cands, err := s.router.Generate(ctx, llm.GenerateRequest{
		Prompt: prompt, N: 1, Modality: llm.ModalityText,
	})
	if err != nil {
		return nil, fmt.Errorf("crystallize llm: %w", err)
	}
	if len(cands) == 0 || strings.TrimSpace(cands[0].Text) == "" {
		return nil, fmt.Errorf("crystallize: empty llm output")
	}
	summary := strings.TrimSpace(cands[0].Text)
	provider := "" // Router 当前未暴露选中的 provider 名；S2.5 加 Recorder 钩子
	cost := cands[0].CostTokens

	// 5. 写 PG
	now := time.Now().UTC()
	p := &domain.PermaNode{
		ID:            platform.NewID(),
		LakeID:        in.LakeID,
		OwnerID:       actor.ID,
		Title:         deriveTitle(titleHint, summary),
		Summary:       summary,
		SourceNodeIDs: append([]string{}, in.SourceNodeIDs...),
		LLMProvider:   provider,
		LLMCostTokens: cost,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := s.perma.Create(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

// deriveTitle 取 hint 或 summary 前 32 字符。
func deriveTitle(hint, summary string) string {
	if hint != "" && hint != "（自动生成）" {
		return truncRunes(hint, 64)
	}
	return truncRunes(summary, 32)
}

func truncRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}
