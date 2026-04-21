// Package httpapi_test · HTTP 集成测试。
//
// 用真实的 PG / Neo4j / Redis 连接装配 router，然后通过 httptest 走完整请求链路。
// 触发条件：环境变量 RIPPLE_INTEGRATION=1 且 PG/Neo4j 可达，否则 t.Skip。
//
// 覆盖：注册→登录→建湖→列表→建节点→蒸发→恢复→凝露
//
// 用法（PowerShell）：
//
//	$env:RIPPLE_INTEGRATION=1
//	cd backend-go
//	# 加载 .env 注入 RIPPLE_PG_URL 等
//	go test ./internal/api/http/... -run TestIntegration -v
package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	httpapi "github.com/chenqilscy/ripple/backend-go/internal/api/http"
	"github.com/chenqilscy/ripple/backend-go/internal/config"
	"github.com/chenqilscy/ripple/backend-go/internal/platform"
	"github.com/chenqilscy/ripple/backend-go/internal/realtime"
	"github.com/chenqilscy/ripple/backend-go/internal/service"
	"github.com/chenqilscy/ripple/backend-go/internal/store"
	"github.com/google/uuid"
)

// integrationFixture 持有所有真实连接，由 setup 构造。
type integrationFixture struct {
	srv  *httptest.Server
	cfg  *config.Config
	tok  string
	user struct {
		ID    string
		Email string
	}
}

func setup(t *testing.T) *integrationFixture {
	t.Helper()
	if os.Getenv("RIPPLE_INTEGRATION") != "1" {
		t.Skip("set RIPPLE_INTEGRATION=1 to enable httpapi integration tests")
	}

	cfg, err := config.Load()
	if err != nil {
		t.Skipf("config load failed (set RIPPLE_PG_URL etc.): %v", err)
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

	authSvc := service.NewAuthService(users, jwt)
	lakeSvc := service.NewLakeService(lakes, memberships, outbox, txRunner)
	broker := realtime.NewMemoryBroker(64)
	t.Cleanup(func() { _ = broker.Close() })
	nodeSvc := service.NewNodeService(nodes, memberships, lakes, broker)

	// Outbox dispatcher：把 PG outbox 事件应用到 Neo4j（建湖→建 :Lake 节点）。
	// 集成测必须启用，否则 ListMine→GetLake 取不到刚建的湖。
	dispLogger := platform.NewLogger("warn", "test")
	dispatcher := service.NewOutboxDispatcher(outbox, lakes, dispLogger)
	dispCtx, dispCancel := context.WithCancel(context.Background())
	go dispatcher.Run(dispCtx)
	t.Cleanup(dispCancel)

	router := httpapi.NewRouter(httpapi.Deps{
		Auth:        authSvc,
		Lakes:       lakeSvc,
		Nodes:       nodeSvc,
		CORSOrigins: []string{"*"},
	})

	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	return &integrationFixture{srv: srv, cfg: cfg}
}

// do 发请求，自动带 Bearer token。返回 status + 反序列化结果。
func (f *integrationFixture) do(t *testing.T, method, path string, body any, out any) int {
	t.Helper()
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, f.srv.URL+path, rd)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if f.tok != "" {
		req.Header.Set("Authorization", "Bearer "+f.tok)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do %s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	if out != nil && resp.StatusCode < 400 {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			t.Fatalf("decode %s %s: %v", method, path, err)
		}
	}
	return resp.StatusCode
}

// TestIntegrationFullFlow 走通完整链路。
func TestIntegrationFullFlow(t *testing.T) {
	f := setup(t)

	email := fmt.Sprintf("itest+%s@ripple.local", uuid.NewString()[:8])
	password := "Test1234!"

	// 1. 注册
	var u struct {
		ID          string `json:"id"`
		Email       string `json:"email"`
		DisplayName string `json:"display_name"`
	}
	if code := f.do(t, "POST", "/api/v1/auth/register", map[string]string{
		"email": email, "password": password, "display_name": "itest",
	}, &u); code != http.StatusCreated {
		t.Fatalf("register: want 201, got %d", code)
	}
	if u.Email != email {
		t.Fatalf("register: email mismatch: got %q", u.Email)
	}
	f.user.ID = u.ID
	f.user.Email = u.Email

	// 2. 登录
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
	if login.AccessToken == "" {
		t.Fatal("login: empty access token")
	}
	f.tok = login.AccessToken

	// 3. /me
	var me struct {
		ID    string `json:"id"`
		Email string `json:"email"`
	}
	if code := f.do(t, "GET", "/api/v1/auth/me", nil, &me); code != http.StatusOK {
		t.Fatalf("me: want 200, got %d", code)
	}
	if me.ID != f.user.ID {
		t.Fatalf("me: id mismatch")
	}

	// 4. 建湖
	var lake struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		OwnerID string `json:"owner_id"`
		Role    string `json:"role"`
	}
	if code := f.do(t, "POST", "/api/v1/lakes", map[string]any{
		"name":      "集成测试湖-" + uuid.NewString()[:6],
		"is_public": false,
	}, &lake); code != http.StatusCreated {
		t.Fatalf("create lake: want 201, got %d", code)
	}
	if lake.OwnerID != f.user.ID {
		t.Fatalf("create lake: owner mismatch")
	}
	if lake.Role != "OWNER" {
		t.Fatalf("create lake: want role OWNER, got %q", lake.Role)
	}

	// 5. 列出我的湖（应包含刚建的；outbox 异步，需轮询）
	var listed struct {
		Lakes []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"lakes"`
	}
	found := false
	for i := 0; i < 30; i++ { // 最多等 ~6s
		listed.Lakes = nil
		if code := f.do(t, "GET", "/api/v1/lakes", nil, &listed); code != http.StatusOK {
			t.Fatalf("list lakes: want 200, got %d", code)
		}
		for _, l := range listed.Lakes {
			if l.ID == lake.ID {
				found = true
				break
			}
		}
		if found {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if !found {
		t.Fatalf("list lakes: created lake not in result after 6s (outbox dispatcher stuck?)")
	}

	// 6. 建节点
	var node struct {
		ID     string `json:"id"`
		LakeID string `json:"lake_id"`
		State  string `json:"state"`
	}
	if code := f.do(t, "POST", "/api/v1/nodes", map[string]any{
		"lake_id": lake.ID,
		"content": "集成测试节点",
		"type":    "TEXT",
	}, &node); code != http.StatusCreated {
		t.Fatalf("create node: want 201, got %d", code)
	}
	if node.State != "DROP" {
		t.Fatalf("create node: want state DROP, got %q", node.State)
	}

	// 7. 列出湖的节点
	var nodeList struct {
		Nodes []struct {
			ID    string `json:"id"`
			State string `json:"state"`
		} `json:"nodes"`
	}
	if code := f.do(t, "GET", "/api/v1/lakes/"+lake.ID+"/nodes", nil, &nodeList); code != http.StatusOK {
		t.Fatalf("list nodes: want 200, got %d", code)
	}
	if len(nodeList.Nodes) == 0 {
		t.Fatal("list nodes: empty")
	}

	// 8. 蒸发节点 → VAPOR
	var vap struct {
		State string `json:"state"`
	}
	if code := f.do(t, "POST", "/api/v1/nodes/"+node.ID+"/evaporate", nil, &vap); code != http.StatusOK {
		t.Fatalf("evaporate: want 200, got %d", code)
	}
	if vap.State != "VAPOR" {
		t.Fatalf("evaporate: want VAPOR, got %q", vap.State)
	}

	// 9. 恢复 → DROP
	var rest struct {
		State string `json:"state"`
	}
	if code := f.do(t, "POST", "/api/v1/nodes/"+node.ID+"/restore", nil, &rest); code != http.StatusOK {
		t.Fatalf("restore: want 200, got %d", code)
	}
	if rest.State != "DROP" {
		t.Fatalf("restore: want DROP, got %q", rest.State)
	}

	// 10. 凝露：必须先有 MIST 节点（直接经造云路径插一个绕不开 LLM）
	//     这里用 service 层直接构造 MIST 不可行（要走仓库），改为：
	//     无 MIST 时跳过 condense，断言 DROP→Condense 必须 400/409 即可。
	var errBody struct {
		Message string `json:"message"`
	}
	code := f.do(t, "POST", "/api/v1/nodes/"+node.ID+"/condense", map[string]string{
		"lake_id": lake.ID,
	}, &errBody)
	if code != http.StatusBadRequest && code != http.StatusConflict && code != http.StatusUnprocessableEntity {
		t.Fatalf("condense on DROP: want 4xx (cannot condense non-MIST), got %d", code)
	}
}

// TestIntegrationAuthGuard 保护端点须带 token。
func TestIntegrationAuthGuard(t *testing.T) {
	f := setup(t)
	// 未登录访问 /me 应 401
	if code := f.do(t, "GET", "/api/v1/auth/me", nil, nil); code != http.StatusUnauthorized {
		t.Fatalf("guard /me: want 401, got %d", code)
	}
	if code := f.do(t, "GET", "/api/v1/lakes", nil, nil); code != http.StatusUnauthorized {
		t.Fatalf("guard /lakes: want 401, got %d", code)
	}
}

// TestIntegrationHealthz 健康检查匿名可达。
func TestIntegrationHealthz(t *testing.T) {
	f := setup(t)
	resp, err := http.Get(f.srv.URL + "/healthz")
	if err != nil {
		t.Fatalf("healthz: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("healthz: want 200, got %d", resp.StatusCode)
	}
}
