// Package httpapi_test · Phase 15 HTTP 集成测试。
//
// 覆盖：PromptTemplate CRUD、AI Trigger 冲突检测、Subscription 套餐 API。
// 触发条件：RIPPLE_INTEGRATION=1 且 PG 可达，否则 t.Skip。
//
// 用法（PowerShell）：
//
//	$env:RIPPLE_INTEGRATION=1
//	cd backend-go ; go test ./internal/api/http/... -run TestIntegrationPhase15 -v
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

// phase15Fixture 持有 Phase 15 专用连接和 repos。
type phase15Fixture struct {
	*integrationFixture
	promptRepo  store.PromptTemplateRepository
	aiJobRepo   store.AiJobRepository
	subRepo     store.SubscriptionRepository
	orgRepo     store.OrgRepository
	analyticsRp store.LLMCallsAnalyticsRepository
	subSvc      *service.SubscriptionService
	orgSvc      *service.OrgService
}

func setup15(t *testing.T) *phase15Fixture {
	t.Helper()
	if os.Getenv("RIPPLE_INTEGRATION") != "1" {
		t.Skip("set RIPPLE_INTEGRATION=1 to enable phase15 integration tests")
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

	// Phase 15 repos & services
	promptRepo := store.NewPromptTemplateRepository(pg)
	aiJobRepo := store.NewAiJobRepository(pg)
	subRepo := store.NewSubscriptionRepository(pg)
	orgQuotaRepo := store.NewOrgQuotaRepository(pg)
	analyticsRepo := store.NewLLMCallsAnalyticsRepository(pg)
	subSvc := service.NewSubscriptionService(subRepo, orgQuotaRepo)
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
		StubPaymentEnabled: true, // 集成测启用 stub 支付
	})

	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	base := &integrationFixture{srv: srv, cfg: cfg}
	return &phase15Fixture{
		integrationFixture: base,
		promptRepo:         promptRepo,
		aiJobRepo:          aiJobRepo,
		subRepo:            subRepo,
		orgRepo:            orgRepo,
		analyticsRp:        analyticsRepo,
		subSvc:             subSvc,
		orgSvc:             orgSvc,
	}
}

// registerAndLogin 快速注册+登录，返回 token。
func (f *phase15Fixture) registerAndLogin(t *testing.T, suffix string) string {
	t.Helper()
	email := fmt.Sprintf("p15%s+%s@ripple.local", suffix, uuid.NewString()[:6])
	password := "Test1234!"
	if code := f.do(t, "POST", "/api/v1/auth/register", map[string]string{
		"email": email, "password": password, "display_name": "p15-" + suffix,
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
	return login.AccessToken
}

// TestIntegrationPhase15PromptTemplate 测试 PromptTemplate CRUD。
func TestIntegrationPhase15PromptTemplate(t *testing.T) {
	f := setup15(t)
	f.tok = f.registerAndLogin(t, "tpl")

	// 1. 创建模板
	var created struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Scope    string `json:"scope"`
		Template string `json:"template"`
	}
	if code := f.do(t, "POST", "/api/v1/prompt_templates", map[string]any{
		"name":     "test-template",
		"template": "Hello {{name}}",
		"scope":    "private",
	}, &created); code != http.StatusCreated {
		t.Fatalf("create template: want 201, got %d", code)
	}
	if created.Name != "test-template" || created.Scope != "private" {
		t.Fatalf("create template: bad fields %+v", created)
	}

	// 2. 列表
	var list struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
		Total int `json:"total"`
	}
	if code := f.do(t, "GET", "/api/v1/prompt_templates", nil, &list); code != http.StatusOK {
		t.Fatalf("list templates: want 200, got %d", code)
	}
	if list.Total < 1 {
		t.Fatalf("list templates: want ≥1, got %d", list.Total)
	}

	// 3. 获取
	var got struct {
		ID string `json:"id"`
	}
	if code := f.do(t, "GET", "/api/v1/prompt_templates/"+created.ID, nil, &got); code != http.StatusOK {
		t.Fatalf("get template: want 200, got %d", code)
	}
	if got.ID != created.ID {
		t.Fatalf("get template: id mismatch")
	}

	// 4. 更新
	var patched struct {
		Name string `json:"name"`
	}
	if code := f.do(t, "PATCH", "/api/v1/prompt_templates/"+created.ID, map[string]any{
		"name": "updated-name",
	}, &patched); code != http.StatusOK {
		t.Fatalf("patch template: want 200, got %d", code)
	}
	if patched.Name != "updated-name" {
		t.Fatalf("patch template: name not updated, got %q", patched.Name)
	}

	// 5. 空 name 应被拒绝
	if code := f.do(t, "PATCH", "/api/v1/prompt_templates/"+created.ID, map[string]any{
		"name": "",
	}, nil); code != http.StatusBadRequest {
		t.Fatalf("patch empty name: want 400, got %d", code)
	}

	// 6. 删除
	if code := f.do(t, "DELETE", "/api/v1/prompt_templates/"+created.ID, nil, nil); code != http.StatusNoContent {
		t.Fatalf("delete template: want 204, got %d", code)
	}

	// 7. 再次获取应 404
	if code := f.do(t, "GET", "/api/v1/prompt_templates/"+created.ID, nil, nil); code != http.StatusNotFound {
		t.Fatalf("get deleted: want 404, got %d", code)
	}
}

// TestIntegrationPhase15AiTrigger 测试 AI Trigger 冲突检测。
func TestIntegrationPhase15AiTrigger(t *testing.T) {
	f := setup15(t)
	f.tok = f.registerAndLogin(t, "aitrigger")

	// 建湖（需要等 outbox）
	var lake struct{ ID string `json:"id"` }
	if code := f.do(t, "POST", "/api/v1/lakes", map[string]any{
		"name":      "ai-lake-" + uuid.NewString()[:6],
		"is_public": false,
	}, &lake); code != http.StatusCreated {
		t.Fatalf("create lake: %d", code)
	}
	// 等 outbox
	for i := 0; i < 30; i++ {
		var l struct{ ID string `json:"id"` }
		if code := f.do(t, "GET", "/api/v1/lakes/"+lake.ID, nil, &l); code == http.StatusOK && l.ID == lake.ID {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	// 建节点
	var node struct{ ID string `json:"id"` }
	if code := f.do(t, "POST", "/api/v1/nodes", map[string]any{
		"lake_id": lake.ID, "content": "ai节点内容", "type": "TEXT",
	}, &node); code != http.StatusCreated {
		t.Fatalf("create node: %d", code)
	}

	// 第一次触发 → 202
	trigPath := fmt.Sprintf("/api/v1/lakes/%s/nodes/%s/ai_trigger", lake.ID, node.ID)
	if code := f.do(t, "POST", trigPath, map[string]any{}, nil); code != http.StatusAccepted {
		t.Fatalf("first trigger: want 202, got %d", code)
	}

	// 第二次触发 → 409（已有活跃任务）
	if code := f.do(t, "POST", trigPath, map[string]any{}, nil); code != http.StatusConflict {
		t.Fatalf("second trigger: want 409, got %d", code)
	}

	// 查询 AI 状态 → 200
	statusPath := fmt.Sprintf("/api/v1/lakes/%s/nodes/%s/ai_status", lake.ID, node.ID)
	var status struct {
		Status   string `json:"status"`
		Progress int    `json:"progress_pct"`
	}
	if code := f.do(t, "GET", statusPath, nil, &status); code != http.StatusOK {
		t.Fatalf("ai status: want 200, got %d", code)
	}
	if status.Status != "pending" && status.Status != "processing" && status.Status != "done" {
		t.Fatalf("ai status: unexpected status %q", status.Status)
	}
}

// TestIntegrationPhase15SubscriptionPlans 测试套餐列表（无鉴权）。
func TestIntegrationPhase15SubscriptionPlans(t *testing.T) {
	f := setup15(t)
	f.tok = f.registerAndLogin(t, "subplans")

	var resp struct {
		Plans []struct {
			ID              string `json:"id"`
			PriceCNYMonthly int    `json:"price_cny_monthly"`
		} `json:"plans"`
	}
	if code := f.do(t, "GET", "/api/v1/subscriptions/plans", nil, &resp); code != http.StatusOK {
		t.Fatalf("get plans: want 200, got %d", code)
	}
	if len(resp.Plans) != 3 {
		t.Fatalf("get plans: want 3 plans, got %d", len(resp.Plans))
	}
	ids := map[string]bool{}
	for _, p := range resp.Plans {
		ids[p.ID] = true
	}
	if !ids["free"] || !ids["pro"] || !ids["team"] {
		t.Fatalf("get plans: missing expected plan ids: %+v", ids)
	}
}

// TestIntegrationPhase15SubscriptionCreateAndGet 测试创建订阅（stub 模式）。
func TestIntegrationPhase15SubscriptionCreateAndGet(t *testing.T) {
	f := setup15(t)
	f.tok = f.registerAndLogin(t, "subsub")

	// 建组织
	var org struct {
		ID string `json:"id"`
	}
	orgSuffix := uuid.NewString()[:6]
	if code := f.do(t, "POST", "/api/v1/organizations", map[string]any{
		"name": "sub-test-org-" + orgSuffix,
		"slug": "subtest" + orgSuffix,
	}, &org); code != http.StatusCreated {
		t.Fatalf("create org: want 201, got %d", code)
	}

	// 订阅 pro（stub_confirm=true）
	var sub struct {
		Subscription struct {
			PlanID string `json:"plan_id"`
			Status string `json:"status"`
		} `json:"subscription"`
	}
	if code := f.do(t, "POST", fmt.Sprintf("/api/v1/organizations/%s/subscription", org.ID), map[string]any{
		"plan_id":      "pro",
		"billing_cycle": "monthly",
		"stub_confirm": true,
	}, &sub); code != http.StatusCreated {
		t.Fatalf("create subscription: want 201, got %d", code)
	}
	if sub.Subscription.PlanID != "pro" || sub.Subscription.Status != "active" {
		t.Fatalf("create subscription: unexpected %+v", sub.Subscription)
	}

	// 查询订阅
	var got struct {
		Subscription struct {
			PlanID string `json:"plan_id"`
		} `json:"subscription"`
	}
	if code := f.do(t, "GET", fmt.Sprintf("/api/v1/organizations/%s/subscription", org.ID), nil, &got); code != http.StatusOK {
		t.Fatalf("get subscription: want 200, got %d", code)
	}
	if got.Subscription.PlanID != "pro" {
		t.Fatalf("get subscription: wrong plan %q", got.Subscription.PlanID)
	}
}
