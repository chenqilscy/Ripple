package httpapi

import (
	"testing"
)

func TestSimpleContentSimilarity(t *testing.T) {
	tests := []struct {
		name   string
		a      string
		b      string
		minSim float64
	}{
		{"完全相同", "系统可用性 扩容 容错", "系统可用性 扩容 容错", 0.99}, // 1.0 but floating point
		{"部分相同", "系统可用性 扩容", "系统可用性 容错", 0.49},
		{"无交集", "系统设计", "产品运营", 0.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sim := simpleContentSimilarity(tt.a, tt.b)
			if sim < tt.minSim {
				t.Errorf("simpleContentSimilarity(%q, %q) = %v, want >= %v", tt.a, tt.b, sim, tt.minSim)
			}
		})
	}
}

func TestGenerateRecommendations(t *testing.T) {
	nodes := []struct {
		ID      string
		Content string
	}{
		{ID: "n1", Content: "系统可用性 扩容"},
		{ID: "n2", Content: "系统可用性 容错"},
		{ID: "n3", Content: "产品运营 数据分析"},
	}
	existingEdges := map[[2]string]bool{}
	recs := generateRecommendations(nodes, existingEdges)

	// n1 和 n2 应该被推荐（有共同词"系统可用性"）
	found := false
	for _, r := range recs {
		if (r.SourceNodeID == "n1" && r.TargetNodeID == "n2") ||
			(r.SourceNodeID == "n2" && r.TargetNodeID == "n1") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected recommendation between n1 and n2, got %d recs", len(recs))
	}

	// n1 和 n3 不应该被推荐（无交集）
	for _, r := range recs {
		if (r.SourceNodeID == "n1" && r.TargetNodeID == "n3") ||
			(r.SourceNodeID == "n3" && r.TargetNodeID == "n1") {
			t.Errorf("should not recommend n1-n3 (no overlap)")
		}
	}
}

func TestGenerateRecommendations_ExistingEdge(t *testing.T) {
	nodes := []struct {
		ID      string
		Content string
	}{
		{ID: "n1", Content: "系统可用性 扩容"},
		{ID: "n2", Content: "系统可用性 容错"},
	}
	existingEdges := map[[2]string]bool{
		[2]string{"n1", "n2"}: true,
	}
	recs := generateRecommendations(nodes, existingEdges)
	if len(recs) > 0 {
		t.Errorf("existing edge should not generate recommendation, got %d", len(recs))
	}
}
