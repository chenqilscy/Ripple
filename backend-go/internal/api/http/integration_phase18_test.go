// Package httpapi_test · Phase 18 HTTP 集成测试。
//
// 覆盖：
//   - P18-B：节点外链分享（创建、公开读取、撤销）
//   - P18-C：节点模板库（创建、列表、从模板建节点）
//   - P18-D：图谱快照（创建、列表）
//
// 触发条件：RIPPLE_INTEGRATION=1 且 PG+Neo4j 可达，否则 t.Skip。
//
// 用法（PowerShell）：
//
//	$env:RIPPLE_INTEGRATION=1
//	cd backend-go ; go test ./internal/api/http/... -run TestIntegrationPhase18 -v
package httpapi_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	httpapi "github.com/chenqilscy/ripple/backend-go/internal/api/http"
	"github.com/chenqilscy/ripple/backend-go/internal/config"
	"github.com/chenqilscy/ripple/backend-go/internal/platform"
	"github.com/chenqilscy/ripple/backend-go/internal/presence"
	"github.com/chenqilscy/ripple/backend-go/internal/realtime"
	"github.com/chenqilscy/ripple/backend-go/internal/service"
	"github.com/chenqilscy/ripple/backend-go/internal/store"
	"github.com/google/uuid"
	"net/http/httptest"
)

// phase18Fixture 持有 Phase 18 专用连接和 repos。
type phase18Fixture struct {
	*integrationFixture
	lakeSvc  *service.LakeService
	nodeSvc  *service.NodeService
	subSvc   *service.SubscriptionService
}

func setup18(t *testing.T) *phase18Fixture {
	t.Helper()
	if os.Getenv("RIPPLE_INTEGRATION") != "1" {
		t.Skip("set RIPPLE_INTEGRATION=1 to enable phase18 integration tests")
	}
	loadIntegrationEnvFromDotEnv()

	cfg, err := config.Load()
	if err != nil {
		t.Skipf("config load failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pg, err := store.NewPGPool(ctx, cfg)
	if err != nil {
		t.Skipf("pg unavailable: %v", err)
	}
	t.Cleanup(pg.Close)

	neo, err := store.NewNeo4jDriver(ctx, cfg)
	if err != nil {
		t.Skipf("neo4j unavailable: %v", err)
	}
	t.Cleanup(func() { _ = neo.Close(context.Background()) })

	jwt := platform.NewJWTSigner(cfg.JWTSecret, cfg.JWTExpiresIn)
	users := store.NewUserRepository(pg)
	memberships := store.NewMembershipRepository(pg)
	outbox := store.NewOutboxRepository(pg)
	txRunner := store.NewTxRunner(pg)
	lakes := store.NewLakeRepository(neo, cfg.Neo4jDatabase)
	nodes := store.NewNodeRepository(neo, cfg.Neo4jDatabase)
	edges := store.NewEdgeRepository(neo, cfg.Neo4jDatabase)
	invites := store.NewInviteRepository(pg)

	authSvc := service.NewAuthService(users, jwt)
	lakeSvc := service.NewLakeService(lakes, memberships, outbox, txRunner)
	orgRepo := store.NewOrgRepository(pg)
	orgSvc := service.NewOrgService(orgRepo)
	broker := realtime.NewMemoryBroker(64)
	t.Cleanup(func() { _ = broker.Close() })
	nodeRevs := store.NewNodeRevisionRepository(pg)
	nodeSvc := service.NewNodeService(nodes, memberships, lakes, broker).WithRevisions(nodeRevs)
	edgeSvc := service.NewEdgeService(edges, nodes, memberships, lakes, broker)
	inviteSvc := service.NewInviteService(invites, memberships, lakes)

	rds, _ := store.NewRedis(ctx, cfg)
	if rds != nil {
		t.Cleanup(func() { _ = rds.Close() })
	}
	presenceSvc := presence.NewService(rds, broker, 0)

	dispLogger := platform.NewLogger("warn", "test")
	dispatcher := service.NewOutboxDispatcher(outbox, lakes, dispLogger)
	dispCtx, dispCancel := context.WithCancel(context.Background())
	go dispatcher.Run(dispCtx)
	t.Cleanup(dispCancel)

	// Phase 15+16 repos
	promptRepo := store.NewPromptTemplateRepository(pg)
	aiJobRepo := store.NewAiJobRepository(pg)
	subRepo := store.NewSubscriptionRepository(pg)
	orgQuotaRepo := store.NewOrgQuotaRepository(pg)
	analyticsRepo := store.NewLLMCallsAnalyticsRepository(pg)
	subSvc := service.NewSubscriptionService(subRepo, orgQuotaRepo).
		WithUsageRepos(orgRepo, lakes, nodes)
	llmUsageSvc := service.NewLLMUsageService(analyticsRepo)

	// Phase 18 repos
	nodeTemplateRepo := store.NewNodeTemplateRepository(pg)
	lakeSnapshotRepo := store.NewLakeSnapshotRepository(pg)
	nodeShareRepo := store.NewNodeShareRepository(pg)

	router := httpapi.NewRouter(httpapi.Deps{
		Auth:               authSvc,
		Lakes:              lakeSvc,
		Nodes:              nodeSvc,
		Edges:              edgeSvc,
		Invites:            inviteSvc,
		Orgs:               orgSvc,
		Users:              users,
		Presence:           presenceSvc,
		WsToken:            &httpapi.WsTokenHandlers{JWT: jwt},
		APIKeys:            store.NewAPIKeyRepository(pg),
		AuditLogs:          store.NewAuditLogRepository(pg),
		Memberships:        memberships,
		CORSOrigins:        []string{"*"},
		AiJobs:             aiJobRepo,
		PromptTemplates:    promptRepo,
		Subscriptions:      subSvc,
		LLMUsage:           llmUsageSvc,
		StubPaymentEnabled: true,
		NodeTemplates:      nodeTemplateRepo,
		LakeSnapshots:      lakeSnapshotRepo,
		NodeShares:         nodeShareRepo,
	})

	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	base := &integrationFixture{srv: srv, cfg: cfg}
	return &phase18Fixture{
		integrationFixture: base,
		lakeSvc:            lakeSvc,
		nodeSvc:            nodeSvc,
		subSvc:             subSvc,
	}
}

// register18AndLogin 注册+登录，返回 (token, userID)。
func (f *phase18Fixture) register18AndLogin(t *testing.T, suffix string) (string, string) {
	t.Helper()
	email := fmt.Sprintf("p18%s+%s@ripple.local", suffix, uuid.NewString()[:6])
	password := "Test1234!"
	if code := f.do(t, "POST", "/api/v1/auth/register", map[string]string{
		"email": email, "password": password, "display_name": "p18-" + suffix,
	}, nil); code != http.StatusCreated {
		t.Fatalf("register: want 201, got %d", code)
	}
	var login struct {
		AccessToken string `json:"access_token"`
		User        struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	if code := f.do(t, "POST", "/api/v1/auth/login", map[string]string{
		"email": email, "password": password,
	}, &login); code != http.StatusOK {
		t.Fatalf("login: want 200, got %d", code)
	}
	f.user.ID = login.User.ID
	return login.AccessToken, login.User.ID
}

// createLakeAndNode 创建湖泊和节点，等待 outbox 处理后返回 lakeID 和 nodeID。
func (f *phase18Fixture) createLakeAndNode(t *testing.T) (lakeID, nodeID string) {
	t.Helper()

	var lake struct {
		ID string `json:"id"`
	}
	lakeName := "p18-lake-" + uuid.NewString()[:6]
	if code := f.do(t, "POST", "/api/v1/lakes", map[string]any{
		"name": lakeName, "is_public": false,
	}, &lake); code != http.StatusCreated {
		t.Fatalf("create lake: want 201, got %d", code)
	}

	// 等待 outbox 处理（最多 30 × 200ms = 6s）
	lakeReady := false
	for i := 0; i < 30; i++ {
		var l struct{ ID string `json:"id"` }
		if code := f.do(t, "GET", "/api/v1/lakes/"+lake.ID, nil, &l); code == http.StatusOK && l.ID == lake.ID {
			lakeReady = true
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if !lakeReady {
		t.Fatal("lake not ready after outbox timeout (6s)")
	}

	var node struct {
		ID string `json:"id"`
	}
	if code := f.do(t, "POST", "/api/v1/nodes", map[string]any{
		"lake_id": lake.ID,
		"content": "Phase18 test node content",
		"type":    "TEXT",
	}, &node); code != http.StatusCreated {
		t.Fatalf("create node: want 201, got %d", code)
	}

	return lake.ID, node.ID
}

// --- P18-B：节点外链分享 ---

// TestIntegrationPhase18ShareCreateAndRead 创建分享链接并公开读取节点。
func TestIntegrationPhase18ShareCreateAndRead(t *testing.T) {
	f := setup18(t)
	f.tok, _ = f.register18AndLogin(t, "share1")
	_, nodeID := f.createLakeAndNode(t)

	// 创建分享（24小时有效）
	var share struct {
		ID      string `json:"id"`
		Token   string `json:"token"`
		PageURL string `json:"page_url"`
		APIURL  string `json:"api_url"`
	}
	if code := f.do(t, "POST", "/api/v1/nodes/"+nodeID+"/share", map[string]any{
		"ttl_hours": 24,
	}, &share); code != http.StatusCreated {
		t.Fatalf("create share: want 201, got %d", code)
	}
	if share.Token == "" {
		t.Fatal("share token must not be empty")
	}
	if share.PageURL == "" {
		t.Fatal("page_url must not be empty")
	}

	// 公开端点访问（无需 token）
	var pubResp struct {
		Node struct {
			ID      string `json:"id"`
			Content string `json:"content"`
		} `json:"node"`
		ShareID string `json:"share_id"`
	}
	oldTok := f.tok
	f.tok = "" // 清除 auth token，模拟匿名访问
	if code := f.do(t, "GET", "/api/v1/share/"+share.Token, nil, &pubResp); code != http.StatusOK {
		t.Fatalf("public share read: want 200, got %d", code)
	}
	f.tok = oldTok

	if pubResp.Node.ID != nodeID {
		t.Fatalf("public share: want node %s, got %s", nodeID, pubResp.Node.ID)
	}
	if pubResp.Node.Content == "" {
		t.Fatal("public share: content must not be empty")
	}
}

// TestIntegrationPhase18ShareListAndRevoke 列出分享并撤销。
func TestIntegrationPhase18ShareListAndRevoke(t *testing.T) {
	f := setup18(t)
	f.tok, _ = f.register18AndLogin(t, "share2")
	_, nodeID := f.createLakeAndNode(t)

	// 创建永久分享
	var share struct {
		ID    string `json:"id"`
		Token string `json:"token"`
	}
	if code := f.do(t, "POST", "/api/v1/nodes/"+nodeID+"/share", map[string]any{
		"ttl_hours": 0,
	}, &share); code != http.StatusCreated {
		t.Fatalf("create share: want 201, got %d", code)
	}

	// 列出分享
	var listResp struct {
		Shares []struct {
			ID    string `json:"id"`
			Token string `json:"token"`
		} `json:"shares"`
	}
	if code := f.do(t, "GET", "/api/v1/nodes/"+nodeID+"/shares", nil, &listResp); code != http.StatusOK {
		t.Fatalf("list shares: want 200, got %d", code)
	}
	found := false
	for _, s := range listResp.Shares {
		if s.ID == share.ID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("created share %s not in list", share.ID)
	}

	// 撤销分享
	if code := f.do(t, "DELETE", "/api/v1/shares/"+share.ID, nil, nil); code != http.StatusNoContent {
		t.Fatalf("revoke share: want 204, got %d", code)
	}

	// 撤销后公开访问应返回 410 Gone
	oldTok := f.tok
	f.tok = ""
	if code := f.do(t, "GET", "/api/v1/share/"+share.Token, nil, nil); code != http.StatusGone {
		t.Fatalf("revoked share: want 410, got %d", code)
	}
	f.tok = oldTok
}

// --- P18-D：图谱快照 ---

// TestIntegrationPhase18SnapshotCreateAndList 创建快照并列出。
func TestIntegrationPhase18SnapshotCreateAndList(t *testing.T) {
	f := setup18(t)
	f.tok, _ = f.register18AndLogin(t, "snap1")
	lakeID, _ := f.createLakeAndNode(t)

	layout := map[string]any{
		"nodes": []map[string]any{
			{"id": "n1", "x": 100, "y": 200},
		},
		"zoom": 1.0,
	}
	layoutBytes, _ := json.Marshal(layout)

	var snap struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		LakeID string `json:"lake_id"`
	}
	if code := f.do(t, "POST", "/api/v1/lakes/"+lakeID+"/snapshots", map[string]any{
		"name":   "布局快照 1",
		"layout": json.RawMessage(layoutBytes),
	}, &snap); code != http.StatusCreated {
		t.Fatalf("create snapshot: want 201, got %d", code)
	}
	if snap.ID == "" {
		t.Fatal("snapshot id must not be empty")
	}
	if snap.LakeID != lakeID {
		t.Fatalf("snapshot lake_id: want %s, got %s", lakeID, snap.LakeID)
	}

	// 列出快照
	var listResp struct {
		Snapshots []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"snapshots"`
	}
	if code := f.do(t, "GET", "/api/v1/lakes/"+lakeID+"/snapshots", nil, &listResp); code != http.StatusOK {
		t.Fatalf("list snapshots: want 200, got %d", code)
	}
	found := false
	for _, s := range listResp.Snapshots {
		if s.ID == snap.ID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("created snapshot %s not in list", snap.ID)
	}
}

// TestIntegrationPhase18SnapshotForbidden 非湖成员无法创建快照。
func TestIntegrationPhase18SnapshotForbidden(t *testing.T) {
	f := setup18(t)
	f.tok, _ = f.register18AndLogin(t, "snapowner")
	lakeID, _ := f.createLakeAndNode(t)

	// 切换到另一个用户
	tok2, _ := f.register18AndLogin(t, "snapother")
	savedTok := f.tok
	f.tok = tok2

	layout := map[string]any{"nodes": []any{}}
	layoutBytes, _ := json.Marshal(layout)
	if code := f.do(t, "POST", "/api/v1/lakes/"+lakeID+"/snapshots", map[string]any{
		"name":   "forbidden-snapshot",
		"layout": json.RawMessage(layoutBytes),
	}, nil); code != http.StatusForbidden {
		t.Fatalf("non-member snapshot: want 403, got %d", code)
	}
	f.tok = savedTok
}

// --- P18-C：节点模板库 ---

// TestIntegrationPhase18TemplateCreateAndList 创建模板并列出。
func TestIntegrationPhase18TemplateCreateAndList(t *testing.T) {
	f := setup18(t)
	f.tok, _ = f.register18AndLogin(t, "tpl1")
	lakeID, _ := f.createLakeAndNode(t)

	// 创建模板
	var tpl struct {
		ID      string   `json:"id"`
		Name    string   `json:"name"`
		Content string   `json:"content"`
		Tags    []string `json:"tags"`
	}
	if code := f.do(t, "POST", "/api/v1/templates", map[string]any{
		"name":        "会议纪要模板",
		"description": "标准会议纪要格式",
		"content":     "## 会议纪要\n\n**日期**: \n**参与者**: \n\n### 议题\n\n### 决议\n",
		"tags":        []string{"会议", "纪要"},
	}, &tpl); code != http.StatusCreated {
		t.Fatalf("create template: want 201, got %d", code)
	}
	if tpl.ID == "" {
		t.Fatal("template id must not be empty")
	}
	if tpl.Name != "会议纪要模板" {
		t.Fatalf("template name: want '会议纪要模板', got '%s'", tpl.Name)
	}

	// 列出模板
	var listResp struct {
		Templates []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"templates"`
	}
	if code := f.do(t, "GET", "/api/v1/templates", nil, &listResp); code != http.StatusOK {
		t.Fatalf("list templates: want 200, got %d", code)
	}
	found := false
	for _, t2 := range listResp.Templates {
		if t2.ID == tpl.ID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("created template %s not in list", tpl.ID)
	}

	// 使用模板在湖中创建节点
	var node struct {
		ID      string `json:"id"`
		Content string `json:"content"`
	}
	if code := f.do(t, "POST", "/api/v1/lakes/"+lakeID+"/nodes/from_template", map[string]any{
		"template_id": tpl.ID,
	}, &node); code != http.StatusCreated {
		t.Fatalf("create node from template: want 201, got %d", code)
	}
	if node.ID == "" {
		t.Fatal("node id from template must not be empty")
	}
	if node.Content != tpl.Content {
		t.Fatalf("node content: want template content, got %q", node.Content)
	}

	// 删除模板
	if code := f.do(t, "DELETE", "/api/v1/templates/"+tpl.ID, nil, nil); code != http.StatusNoContent {
		t.Fatalf("delete template: want 204, got %d", code)
	}
}
