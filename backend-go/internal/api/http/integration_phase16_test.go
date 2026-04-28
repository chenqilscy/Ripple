// Package httpapi_test · Phase 16 HTTP 集成测试。
//
// 覆盖：GET /api/v1/organizations/{id}/usage 真实用量统计。
// 触发条件：RIPPLE_INTEGRATION=1 且 PG+Neo4j 可达，否则 t.Skip。
//
// 用法（PowerShell）：
//
//	$env:RIPPLE_INTEGRATION=1
//	cd backend-go ; go test ./internal/api/http/... -run TestIntegrationPhase16 -v
package httpapi_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
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
)

// phase16Fixture 持有 Phase 16 专用连接和 repos。
type phase16Fixture struct {
	*integrationFixture
	orgRepo  store.OrgRepository
	orgSvc   *service.OrgService
	subSvc   *service.SubscriptionService
	lakeSvc  *service.LakeService
	nodeSvc  *service.NodeService
}

func setup16(t *testing.T) *phase16Fixture {
	t.Helper()
	if os.Getenv("RIPPLE_INTEGRATION") != "1" {
		t.Skip("set RIPPLE_INTEGRATION=1 to enable phase16 integration tests")
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

	// Phase 15+16 repos & services
	promptRepo := store.NewPromptTemplateRepository(pg)
	aiJobRepo := store.NewAiJobRepository(pg)
	subRepo := store.NewSubscriptionRepository(pg)
	orgQuotaRepo := store.NewOrgQuotaRepository(pg)
	analyticsRepo := store.NewLLMCallsAnalyticsRepository(pg)
	// Phase 16: 注入真实用量仓储
	subSvc := service.NewSubscriptionService(subRepo, orgQuotaRepo).
		WithUsageRepos(orgRepo, lakes, nodes)
	llmUsageSvc := service.NewLLMUsageService(analyticsRepo)

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
	})

	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	base := &integrationFixture{srv: srv, cfg: cfg}
	return &phase16Fixture{
		integrationFixture: base,
		orgRepo:            orgRepo,
		orgSvc:             orgSvc,
		subSvc:             subSvc,
		lakeSvc:            lakeSvc,
		nodeSvc:            nodeSvc,
	}
}

// register16AndLogin 注册+登录，返回 (token, userID)。
func (f *phase16Fixture) register16AndLogin(t *testing.T, suffix string) (string, string) {
	t.Helper()
	email := fmt.Sprintf("p16%s+%s@ripple.local", suffix, uuid.NewString()[:6])
	password := "Test1234!"
	if code := f.do(t, "POST", "/api/v1/auth/register", map[string]string{
		"email": email, "password": password, "display_name": "p16-" + suffix,
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

// TestIntegrationPhase16OrgUsageEmpty 验证新组织的用量全为 0。
func TestIntegrationPhase16OrgUsageEmpty(t *testing.T) {
	f := setup16(t)
	f.tok, _ = f.register16AndLogin(t, "usage0")

	// 建组织
	var org struct {
		ID string `json:"id"`
	}
	slug := "p16empty" + uuid.NewString()[:6]
	if code := f.do(t, "POST", "/api/v1/organizations", map[string]any{
		"name": "usage-empty-org",
		"slug": slug,
	}, &org); code != http.StatusCreated {
		t.Fatalf("create org: want 201, got %d", code)
	}

	// 查询用量
	var resp struct {
		Usage struct {
			Members int64 `json:"members"`
			Lakes   int64 `json:"lakes"`
			Nodes   int64 `json:"nodes"`
		} `json:"usage"`
	}
	if code := f.do(t, "GET", "/api/v1/organizations/"+org.ID+"/usage", nil, &resp); code != http.StatusOK {
		t.Fatalf("get usage: want 200, got %d", code)
	}
	// OWNER 自身算1个成员
	if resp.Usage.Members < 1 {
		t.Fatalf("usage.members: want ≥1 (OWNER), got %d", resp.Usage.Members)
	}
	if resp.Usage.Lakes != 0 {
		t.Fatalf("usage.lakes: want 0, got %d", resp.Usage.Lakes)
	}
	if resp.Usage.Nodes != 0 {
		t.Fatalf("usage.nodes: want 0, got %d", resp.Usage.Nodes)
	}
}

// TestIntegrationPhase16OrgUsageWithData 创建湖+节点后验证用量正确递增。
func TestIntegrationPhase16OrgUsageWithData(t *testing.T) {
	f := setup16(t)
	f.tok, _ = f.register16AndLogin(t, "usagedata")

	// 建组织
	var org struct {
		ID string `json:"id"`
	}
	slug := "p16data" + uuid.NewString()[:6]
	if code := f.do(t, "POST", "/api/v1/organizations", map[string]any{
		"name": "usage-data-org",
		"slug": slug,
	}, &org); code != http.StatusCreated {
		t.Fatalf("create org: want 201, got %d", code)
	}

	// 建湖（org_id 关联）
	var lake struct {
		ID string `json:"id"`
	}
	if code := f.do(t, "POST", "/api/v1/lakes", map[string]any{
		"name":      "usage-lake-" + uuid.NewString()[:6],
		"is_public": false,
		"org_id":    org.ID,
	}, &lake); code != http.StatusCreated {
		t.Fatalf("create lake: want 201, got %d", code)
	}

	// 等 outbox 处理（最多 30 × 200ms = 6s）
	for i := 0; i < 30; i++ {
		var l struct{ ID string `json:"id"` }
		if code := f.do(t, "GET", "/api/v1/lakes/"+lake.ID, nil, &l); code == http.StatusOK && l.ID == lake.ID {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	// 建2个节点
	for range 2 {
		var node struct{ ID string `json:"id"` }
		if code := f.do(t, "POST", "/api/v1/nodes", map[string]any{
			"lake_id": lake.ID,
			"content": "test node " + uuid.NewString()[:4],
			"type":    "TEXT",
		}, &node); code != http.StatusCreated {
			t.Fatalf("create node: want 201, got %d", code)
		}
	}

	// 查询用量，验证湖计数 ≥ 1，节点计数 ≥ 2
	var resp struct {
		Usage struct {
			Members int64 `json:"members"`
			Lakes   int64 `json:"lakes"`
			Nodes   int64 `json:"nodes"`
		} `json:"usage"`
	}
	if code := f.do(t, "GET", "/api/v1/organizations/"+org.ID+"/usage", nil, &resp); code != http.StatusOK {
		t.Fatalf("get usage: want 200, got %d", code)
	}
	if resp.Usage.Members < 1 {
		t.Fatalf("usage.members: want ≥1, got %d", resp.Usage.Members)
	}
	if resp.Usage.Lakes < 1 {
		t.Fatalf("usage.lakes: want ≥1, got %d", resp.Usage.Lakes)
	}
	if resp.Usage.Nodes < 2 {
		t.Fatalf("usage.nodes: want ≥2, got %d", resp.Usage.Nodes)
	}
}

// TestIntegrationPhase16OrgUsageForbidden 非成员无权查询用量。
func TestIntegrationPhase16OrgUsageForbidden(t *testing.T) {
	f := setup16(t)
	f.tok, _ = f.register16AndLogin(t, "owner16")

	// 建组织（OWNER 用户）
	var org struct {
		ID string `json:"id"`
	}
	slug := "p16forb" + uuid.NewString()[:6]
	if code := f.do(t, "POST", "/api/v1/organizations", map[string]any{
		"name": "usage-forb-org",
		"slug": slug,
	}, &org); code != http.StatusCreated {
		t.Fatalf("create org: want 201, got %d", code)
	}

	// 另一个用户（非成员）
	tok2, _ := f.register16AndLogin(t, "outsider16")
	origTok := f.tok
	f.tok = tok2

	if code := f.do(t, "GET", "/api/v1/organizations/"+org.ID+"/usage", nil, nil); code != http.StatusForbidden {
		t.Fatalf("outsider usage: want 403, got %d", code)
	}

	f.tok = origTok
}
