// Package httpapi_test · Phase 15 HTTP 集成测试。
//
// 覆盖：PromptTemplate CRUD、AI Trigger 冲突检测、Subscription 套餐 API、LLM 用量账单 API。
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
	"math"
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
	"github.com/jackc/pgx/v5/pgxpool"
)

// phase15Fixture 持有 Phase 15 专用连接和 repos。
type phase15Fixture struct {
	*integrationFixture
	promptRepo  store.PromptTemplateRepository
	aiJobRepo   store.AiJobRepository
	subRepo     store.SubscriptionRepository
	orgRepo     store.OrgRepository
	analyticsRp store.LLMCallsAnalyticsRepository
	pg          *pgxpool.Pool
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
		pg:                 pg,
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

func TestIntegrationPhase15PromptTemplateOrgVisibilityAndAITrigger(t *testing.T) {
	f := setup15(t)

	ownerTok := f.registerAndLogin(t, "tplowner")
	ownerID := f.user.ID
	memberTok := f.registerAndLogin(t, "tplmember")
	memberID := f.user.ID
	outsiderTok := f.registerAndLogin(t, "tploutsider")

	f.tok = ownerTok
	var org struct{ ID string `json:"id"` }
	orgSuffix := uuid.NewString()[:6]
	if code := f.do(t, "POST", "/api/v1/organizations", map[string]any{
		"name": "tpl-org-" + orgSuffix,
		"slug": "tplorg" + orgSuffix,
	}, &org); code != http.StatusCreated {
		t.Fatalf("create org: want 201, got %d", code)
	}
	if code := f.do(t, "POST", fmt.Sprintf("/api/v1/organizations/%s/members", org.ID), map[string]any{
		"user_id": memberID,
		"role":    "MEMBER",
	}, nil); code != http.StatusNoContent {
		t.Fatalf("add org member: want 204, got %d", code)
	}

	var shared struct{ ID string `json:"id"` }
	if code := f.do(t, "POST", "/api/v1/prompt_templates", map[string]any{
		"name":     "shared-template",
		"template": "Hello {{lake_name}}",
		"scope":    "org",
		"org_id":   org.ID,
	}, &shared); code != http.StatusCreated {
		t.Fatalf("create shared template: want 201, got %d", code)
	}

	f.tok = memberTok
	var memberList struct {
		Items []struct{ ID string `json:"id"` } `json:"items"`
	}
	if code := f.do(t, "GET", "/api/v1/prompt_templates", nil, &memberList); code != http.StatusOK {
		t.Fatalf("member list templates: want 200, got %d", code)
	}
	foundShared := false
	for _, item := range memberList.Items {
		if item.ID == shared.ID {
			foundShared = true
			break
		}
	}
	if !foundShared {
		t.Fatalf("member list templates: expected shared template %s", shared.ID)
	}
	if code := f.do(t, "GET", "/api/v1/prompt_templates/"+shared.ID, nil, nil); code != http.StatusOK {
		t.Fatalf("member get shared template: want 200, got %d", code)
	}

	f.tok = outsiderTok
	var outsiderList struct {
		Items []struct{ ID string `json:"id"` } `json:"items"`
	}
	if code := f.do(t, "GET", "/api/v1/prompt_templates", nil, &outsiderList); code != http.StatusOK {
		t.Fatalf("outsider list templates: want 200, got %d", code)
	}
	for _, item := range outsiderList.Items {
		if item.ID == shared.ID {
			t.Fatalf("outsider should not see shared template %s", shared.ID)
		}
	}
	if code := f.do(t, "GET", "/api/v1/prompt_templates/"+shared.ID, nil, nil); code != http.StatusForbidden {
		t.Fatalf("outsider get shared template: want 403, got %d", code)
	}

	f.tok = memberTok
	var memberLake struct{ ID string `json:"id"` }
	if code := f.do(t, "POST", "/api/v1/lakes", map[string]any{
		"name":      "member-ai-lake-" + uuid.NewString()[:6],
		"is_public": false,
	}, &memberLake); code != http.StatusCreated {
		t.Fatalf("member create lake: want 201, got %d", code)
	}
	for i := 0; i < 30; i++ {
		var l struct{ ID string `json:"id"` }
		if code := f.do(t, "GET", "/api/v1/lakes/"+memberLake.ID, nil, &l); code == http.StatusOK && l.ID == memberLake.ID {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	var memberNode struct{ ID string `json:"id"` }
	if code := f.do(t, "POST", "/api/v1/nodes", map[string]any{
		"lake_id": memberLake.ID, "content": "member node", "type": "TEXT",
	}, &memberNode); code != http.StatusCreated {
		t.Fatalf("member create node: want 201, got %d", code)
	}
	memberTriggerPath := fmt.Sprintf("/api/v1/lakes/%s/nodes/%s/ai_trigger", memberLake.ID, memberNode.ID)
	if code := f.do(t, "POST", memberTriggerPath, map[string]any{"prompt_template_id": shared.ID}, nil); code != http.StatusAccepted {
		t.Fatalf("member ai trigger with shared template: want 202, got %d", code)
	}

	f.tok = outsiderTok
	var outsiderLake struct{ ID string `json:"id"` }
	if code := f.do(t, "POST", "/api/v1/lakes", map[string]any{
		"name":      "outsider-ai-lake-" + uuid.NewString()[:6],
		"is_public": false,
	}, &outsiderLake); code != http.StatusCreated {
		t.Fatalf("outsider create lake: want 201, got %d", code)
	}
	for i := 0; i < 30; i++ {
		var l struct{ ID string `json:"id"` }
		if code := f.do(t, "GET", "/api/v1/lakes/"+outsiderLake.ID, nil, &l); code == http.StatusOK && l.ID == outsiderLake.ID {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	var outsiderNode struct{ ID string `json:"id"` }
	if code := f.do(t, "POST", "/api/v1/nodes", map[string]any{
		"lake_id": outsiderLake.ID, "content": "outsider node", "type": "TEXT",
	}, &outsiderNode); code != http.StatusCreated {
		t.Fatalf("outsider create node: want 201, got %d", code)
	}
	outsiderTriggerPath := fmt.Sprintf("/api/v1/lakes/%s/nodes/%s/ai_trigger", outsiderLake.ID, outsiderNode.ID)
	if code := f.do(t, "POST", outsiderTriggerPath, map[string]any{"prompt_template_id": shared.ID}, nil); code != http.StatusForbidden {
		t.Fatalf("outsider ai trigger with shared template: want 403, got %d", code)
	}

	if ownerID == "" {
		t.Fatalf("owner id should not be empty")
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

// TestIntegrationPhase15LLMUsage 测试组织 LLM 用量聚合与成本估算。
func TestIntegrationPhase15LLMUsage(t *testing.T) {
	f := setup15(t)
	f.tok = f.registerAndLogin(t, "llmusage")

	var org struct {
		ID string `json:"id"`
	}
	orgSuffix := uuid.NewString()[:6]
	if code := f.do(t, "POST", "/api/v1/organizations", map[string]any{
		"name": "llm-usage-org-" + orgSuffix,
		"slug": "llmusage" + orgSuffix,
	}, &org); code != http.StatusCreated {
		t.Fatalf("create org: want 201, got %d", code)
	}

	now := time.Now().UTC().Truncate(time.Second)
	yesterday := now.Add(-24 * time.Hour)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	const insertLLMCall = `
INSERT INTO llm_calls (provider, modality, prompt_hash, candidates_n, cost_tokens, latency_ms, status, error_message, created_at, org_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
`
	rows := []struct {
		provider  string
		modality  string
		hash      string
		latencyMS int
		createdAt time.Time
	}{
		{provider: "zhipu", modality: "text", hash: "hash-zhipu-1", latencyMS: 1000, createdAt: now},
		{provider: "zhipu", modality: "text", hash: "hash-zhipu-2", latencyMS: 2000, createdAt: now},
		{provider: "deepseek", modality: "text", hash: "hash-deepseek-1", latencyMS: 900, createdAt: yesterday},
	}
	for _, row := range rows {
		if _, err := f.pg.Exec(ctx, insertLLMCall,
			row.provider, row.modality, row.hash,
			1, 128, row.latencyMS,
			"ok", "", row.createdAt, org.ID,
		); err != nil {
			t.Fatalf("insert llm_call: %v", err)
		}
	}

	var usage struct {
		OrgID                 string  `json:"org_id"`
		PeriodDays            int     `json:"period_days"`
		TotalCalls            int64   `json:"total_calls"`
		TotalEstimatedCostCNY float64 `json:"total_estimated_cost_cny"`
		ByProvider            []struct {
			Provider         string  `json:"provider"`
			Calls            int64   `json:"calls"`
			AvgDurationMS    int64   `json:"avg_duration_ms"`
			EstimatedCostCNY float64 `json:"estimated_cost_cny"`
		} `json:"by_provider"`
		ByDay []struct {
			Date             string  `json:"date"`
			Calls            int64   `json:"calls"`
			EstimatedCostCNY float64 `json:"estimated_cost_cny"`
		} `json:"by_day"`
	}
	if code := f.do(t, "GET", fmt.Sprintf("/api/v1/organizations/%s/llm_usage?days=7", org.ID), nil, &usage); code != http.StatusOK {
		t.Fatalf("get llm usage: want 200, got %d", code)
	}
	if usage.OrgID != org.ID || usage.PeriodDays != 7 {
		t.Fatalf("get llm usage: bad envelope %+v", usage)
	}
	if usage.TotalCalls != 3 {
		t.Fatalf("get llm usage: want total_calls=3, got %d", usage.TotalCalls)
	}
	if math.Abs(usage.TotalEstimatedCostCNY-0.028) > 0.000001 {
		t.Fatalf("get llm usage: want total_estimated_cost_cny=0.028, got %f", usage.TotalEstimatedCostCNY)
	}

	providerMap := map[string]struct {
		calls int64
		avgMS int64
		cost  float64
	}{
		"zhipu":    {calls: 2, avgMS: 1500, cost: 0.02},
		"deepseek": {calls: 1, avgMS: 900, cost: 0.008},
	}
	if len(usage.ByProvider) != len(providerMap) {
		t.Fatalf("get llm usage: want %d providers, got %d", len(providerMap), len(usage.ByProvider))
	}
	for _, got := range usage.ByProvider {
		want, ok := providerMap[got.Provider]
		if !ok {
			t.Fatalf("get llm usage: unexpected provider %q", got.Provider)
		}
		if got.Calls != want.calls || got.AvgDurationMS != want.avgMS || math.Abs(got.EstimatedCostCNY-want.cost) > 0.000001 {
			t.Fatalf("get llm usage: unexpected provider aggregate %+v", got)
		}
	}

	dayMap := map[string]struct {
		calls int64
		cost  float64
	}{
		now.Format("2006-01-02"):       {calls: 2, cost: 0.02},
		yesterday.Format("2006-01-02"): {calls: 1, cost: 0.008},
	}
	if len(usage.ByDay) != len(dayMap) {
		t.Fatalf("get llm usage: want %d day rows, got %d", len(dayMap), len(usage.ByDay))
	}
	for _, got := range usage.ByDay {
		want, ok := dayMap[got.Date]
		if !ok {
			t.Fatalf("get llm usage: unexpected day %q", got.Date)
		}
		if got.Calls != want.calls || math.Abs(got.EstimatedCostCNY-want.cost) > 0.000001 {
			t.Fatalf("get llm usage: unexpected day aggregate %+v", got)
		}
	}
}
