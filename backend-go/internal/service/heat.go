package service

import (
	"context"
	"math"
	"sort"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
)

// HeatWindowDays is the time window (in days) for heat trend calculation.
const HeatWindowDays = 7
// 需要 NodeService（用于 ListByLake 获取节点）、NodeRevisionRepository（用于编辑热度统计）、
// 以及 EdgeService（用于关联热度统计）。
type HeatService struct {
	nodeSvc interface {
		ListByLake(ctx context.Context, actor *domain.User, lakeID string, includeVapor bool) ([]domain.Node, error)
	}
	nodeRevRepo interface {
		CountByNodeIDsSince(ctx context.Context, nodeIDs []string, since time.Time) (map[string]int, error)
	}
	edgeSvc interface {
		ListByLake(ctx context.Context, actor *domain.User, lakeID string, includeDeleted bool) ([]domain.Edge, error)
	}
}

// NewHeatService 构造。
func NewHeatService(
	nodeSvc interface {
		ListByLake(ctx context.Context, actor *domain.User, lakeID string, includeVapor bool) ([]domain.Node, error)
	},
	nodeRevRepo interface {
		CountByNodeIDsSince(ctx context.Context, nodeIDs []string, since time.Time) (map[string]int, error)
	},
	edgeSvc interface {
		ListByLake(ctx context.Context, actor *domain.User, lakeID string, includeDeleted bool) ([]domain.Edge, error)
	},
) *HeatService {
	return &HeatService{
		nodeSvc:    nodeSvc,
		nodeRevRepo: nodeRevRepo,
		edgeSvc:    edgeSvc,
	}
}

// HeatNode 单个热度节点。
type HeatNode struct {
	NodeID           string
	Content          string
	ContentPreview   string
	HeatScore        float64 // 归一化 [0,1]
	EditingScore     float64
	AssociationScore float64
	EditCount        int
	EdgeCount        int
	Rank             int
}

// ComputeHeat 计算 lakeID 下所有节点近 7 天热度。
func (s *HeatService) ComputeHeat(ctx context.Context, actor *domain.User, lakeID string, limit int) ([]HeatNode, error) {
	since := time.Now().UTC().AddDate(0, 0, -HeatWindowDays)

	// 获取所有活跃节点
	nodes, err := s.nodeSvc.ListByLake(ctx, actor, lakeID, true)
	if err != nil {
		return nil, err
	}

	// 过滤活跃节点（排除 ERASED / GHOST）
	var activeNodes []domain.Node
	for _, n := range nodes {
		if n.State != domain.StateErased && n.State != domain.StateGhost {
			activeNodes = append(activeNodes, n)
		}
	}
	if len(activeNodes) < 3 {
		return []HeatNode{}, nil
	}

	nodeIDs := make([]string, len(activeNodes))
	nodeMap := make(map[string]string, len(activeNodes))
	for i, n := range activeNodes {
		nodeIDs[i] = n.ID
		content := n.Content
		if len(content) > 50 {
			content = content[:50] + "..."
		}
		nodeMap[n.ID] = content
	}

	// 编辑热度：查询 node_revisions
	editCounts, err := s.nodeRevRepo.CountByNodeIDsSince(ctx, nodeIDs, since)
	if err != nil {
		return nil, err
	}

	// 关联热度：查询 edges
	edges, err := s.edgeSvc.ListByLake(ctx, actor, lakeID, false)
	if err != nil {
		return nil, err
	}

	edgeCounts := make(map[string]int)
	for _, e := range edges {
		if e.DeletedAt != nil {
			continue
		}
		if e.CreatedAt.Before(since) {
			continue
		}
		edgeCounts[e.SrcNodeID]++
		edgeCounts[e.DstNodeID]++
	}

	// 构建节点热度列表（原始分数）
	type rawScore struct {
		editing float64
		assoc   float64
	}
	raw := make(map[string]rawScore)
	for _, nid := range nodeIDs {
		raw[nid] = rawScore{
			editing: float64(editCounts[nid]),
			assoc:   float64(edgeCounts[nid]),
		}
	}

	// max-min 归一化
	maxEdit, maxAssoc := 1.0, 1.0
	for _, v := range raw {
		if v.editing > maxEdit {
			maxEdit = v.editing
		}
		if v.assoc > maxAssoc {
			maxAssoc = v.assoc
		}
	}

	type scoredNode struct {
		nodeID  string
		content string
		editing float64
		assoc   float64
		heat    float64
	}
	var scored []scoredNode
	for _, nid := range nodeIDs {
		eScore := 0.0
		aScore := 0.0
		if maxEdit > 0 {
			eScore = raw[nid].editing / maxEdit
		}
		if maxAssoc > 0 {
			aScore = raw[nid].assoc / maxAssoc
		}
		heat := eScore*0.6 + aScore*0.4
		scored = append(scored, scoredNode{
			nodeID:  nid,
			content: nodeMap[nid],
			editing: eScore,
			assoc:   aScore,
			heat:    heat,
		})
	}

	// 按热度降序排序（O(n log n)）
	sort.Slice(scored, func(i, j int) bool { return scored[i].heat > scored[j].heat })

	if limit <= 0 {
		limit = 10
	}
	if limit > len(scored) {
		limit = len(scored)
	}

	out := make([]HeatNode, limit)
	for i := 0; i < limit; i++ {
		out[i] = HeatNode{
			NodeID:           scored[i].nodeID,
			Content:          scored[i].content,
			ContentPreview:   scored[i].content,
			HeatScore:        math.Round(scored[i].heat*100) / 100,
			EditingScore:     math.Round(scored[i].editing*100) / 100,
			AssociationScore: math.Round(scored[i].assoc*100) / 100,
			EditCount:        editCounts[scored[i].nodeID],
			EdgeCount:        edgeCounts[scored[i].nodeID],
			Rank:             i + 1,
		}
	}

	return out, nil
}