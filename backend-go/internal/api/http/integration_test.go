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
	"github.com/chenqilscy/ripple/backend-go/internal/presence"
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
	edges := store.NewEdgeRepository(neo, cfg.Neo4jDatabase)
	invites := store.NewInviteRepository(pg)

	authSvc := service.NewAuthService(users, jwt)
	lakeSvc := service.NewLakeService(lakes, memberships, outbox, txRunner)
	broker := realtime.NewMemoryBroker(64)
	t.Cleanup(func() { _ = broker.Close() })
	nodeRevs := store.NewNodeRevisionRepository(pg)
	nodeSvc := service.NewNodeService(nodes, memberships, lakes, broker).WithRevisions(nodeRevs)
	edgeSvc := service.NewEdgeService(edges, nodes, memberships, lakes, broker)
	inviteSvc := service.NewInviteService(invites, memberships, lakes)

	// Redis（可选，没有时 presence 走内存存储）
	rds, _ := store.NewRedis(ctx, cfg)
	if rds != nil {
		t.Cleanup(func() { _ = rds.Close() })
	}
	presenceSvc := presence.NewService(rds, broker, 0)

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
		Edges:       edgeSvc,
		Invites:     inviteSvc,
		Presence:    presenceSvc,
		WsToken:     &httpapi.WsTokenHandlers{JWT: jwt},
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

// TestIntegrationEdgeFlow 走通边的 CRUD：建两节点 → 建边 → 列表 → 删边 → 列表为空。
func TestIntegrationEdgeFlow(t *testing.T) {
	f := setup(t)

	// 注册 + 登录
	email := fmt.Sprintf("edge+%s@ripple.local", uuid.NewString()[:8])
	password := "Test1234!"
	if code := f.do(t, "POST", "/api/v1/auth/register", map[string]string{
		"email": email, "password": password, "display_name": "edge",
	}, nil); code != http.StatusCreated {
		t.Fatalf("register: %d", code)
	}
	var login struct {
		AccessToken string `json:"access_token"`
	}
	if code := f.do(t, "POST", "/api/v1/auth/login", map[string]string{
		"email": email, "password": password,
	}, &login); code != http.StatusOK {
		t.Fatalf("login: %d", code)
	}
	f.tok = login.AccessToken

	// 建湖
	var lake struct{ ID string `json:"id"` }
	if code := f.do(t, "POST", "/api/v1/lakes", map[string]any{
		"name":      "edge-lake-" + uuid.NewString()[:6],
		"is_public": false,
	}, &lake); code != http.StatusCreated {
		t.Fatalf("create lake: %d", code)
	}

	// 等 outbox 把 lake 投到 Neo4j
	for i := 0; i < 30; i++ {
		var l struct{ ID string `json:"id"` }
		if code := f.do(t, "GET", "/api/v1/lakes/"+lake.ID, nil, &l); code == http.StatusOK && l.ID == lake.ID {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	// 建两个节点
	mkNode := func(content string) string {
		var n struct{ ID string `json:"id"` }
		if code := f.do(t, "POST", "/api/v1/nodes", map[string]any{
			"lake_id": lake.ID, "content": content, "type": "TEXT",
		}, &n); code != http.StatusCreated {
			t.Fatalf("create node %s: %d", content, code)
		}
		return n.ID
	}
	src := mkNode("源节点")
	dst := mkNode("目标节点")

	// 建边
	var edge struct {
		ID        string `json:"id"`
		LakeID    string `json:"lake_id"`
		SrcNodeID string `json:"src_node_id"`
		DstNodeID string `json:"dst_node_id"`
		Kind      string `json:"kind"`
	}
	if code := f.do(t, "POST", "/api/v1/edges", map[string]any{
		"src_node_id": src, "dst_node_id": dst, "kind": "relates",
	}, &edge); code != http.StatusCreated {
		t.Fatalf("create edge: %d", code)
	}
	if edge.SrcNodeID != src || edge.DstNodeID != dst || edge.LakeID != lake.ID {
		t.Fatalf("edge fields wrong: %+v", edge)
	}

	// 列表应有 1 条
	var listResp struct {
		Edges []struct {
			ID string `json:"id"`
		} `json:"edges"`
	}
	if code := f.do(t, "GET", "/api/v1/lakes/"+lake.ID+"/edges", nil, &listResp); code != http.StatusOK {
		t.Fatalf("list edges: %d", code)
	}
	if len(listResp.Edges) != 1 || listResp.Edges[0].ID != edge.ID {
		t.Fatalf("list want 1, got %+v", listResp)
	}

	// 重复创建应 409
	if code := f.do(t, "POST", "/api/v1/edges", map[string]any{
		"src_node_id": src, "dst_node_id": dst, "kind": "relates",
	}, nil); code != http.StatusConflict {
		t.Fatalf("duplicate create: want 409, got %d", code)
	}

	// 自环应 400
	if code := f.do(t, "POST", "/api/v1/edges", map[string]any{
		"src_node_id": src, "dst_node_id": src, "kind": "relates",
	}, nil); code != http.StatusBadRequest {
		t.Fatalf("self loop: want 400, got %d", code)
	}

	// 删边
	if code := f.do(t, "DELETE", "/api/v1/edges/"+edge.ID, nil, nil); code != http.StatusNoContent {
		t.Fatalf("delete edge: want 204, got %d", code)
	}

	// 列表应空
	listResp.Edges = nil
	if code := f.do(t, "GET", "/api/v1/lakes/"+lake.ID+"/edges", nil, &listResp); code != http.StatusOK {
		t.Fatalf("list after delete: %d", code)
	}
	if len(listResp.Edges) != 0 {
		t.Fatalf("after delete want empty, got %d", len(listResp.Edges))
	}

	// 再删一次应幂等成功
	if code := f.do(t, "DELETE", "/api/v1/edges/"+edge.ID, nil, nil); code != http.StatusNoContent {
		t.Fatalf("idempotent delete: want 204, got %d", code)
	}
}

// TestIntegrationInviteFlow 走通邀请链路：
//  1. user A 建湖、签发邀请
//  2. user B 用 token 接受 → 成为 PASSENGER
//  3. user B 可访问该湖
//  4. 重复使用同 token（max_uses=1）应失败
//  5. 撤销后再接受应失败
func TestIntegrationInviteFlow(t *testing.T) {
	f := setup(t)

	// ---- user A 注册+登录+建湖 ----
	emailA := fmt.Sprintf("inviter+%s@ripple.local", uuid.NewString()[:8])
	passwordA := "Test1234!"
	if code := f.do(t, "POST", "/api/v1/auth/register", map[string]string{
		"email": emailA, "password": passwordA, "display_name": "A",
	}, nil); code != http.StatusCreated {
		t.Fatalf("A register: %d", code)
	}
	var loginA struct{ AccessToken string `json:"access_token"` }
	if code := f.do(t, "POST", "/api/v1/auth/login", map[string]string{
		"email": emailA, "password": passwordA,
	}, &loginA); code != http.StatusOK {
		t.Fatalf("A login: %d", code)
	}
	f.tok = loginA.AccessToken

	var lake struct{ ID string `json:"id"` }
	if code := f.do(t, "POST", "/api/v1/lakes", map[string]any{
		"name": "invite-lake-" + uuid.NewString()[:6], "is_public": false,
	}, &lake); code != http.StatusCreated {
		t.Fatalf("create lake: %d", code)
	}

	// 等 outbox 投到 Neo4j（preview 里的 lakes.GetByID 需要）
	for i := 0; i < 30; i++ {
		var l struct{ ID string `json:"id"` }
		if code := f.do(t, "GET", "/api/v1/lakes/"+lake.ID, nil, &l); code == http.StatusOK && l.ID == lake.ID {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	// 签发邀请（max_uses=1, ttl=1h, role=PASSENGER）
	var invite struct {
		ID    string `json:"id"`
		Token string `json:"token"`
	}
	if code := f.do(t, "POST", "/api/v1/lakes/"+lake.ID+"/invites", map[string]any{
		"role": "PASSENGER", "max_uses": 1, "ttl_seconds": 3600,
	}, &invite); code != http.StatusCreated {
		t.Fatalf("create invite: %d", code)
	}
	if invite.Token == "" {
		t.Fatal("invite token empty")
	}

	// 列表（A 可见）
	var invList struct {
		Invites []struct {
			ID string `json:"id"`
		} `json:"invites"`
	}
	if code := f.do(t, "GET", "/api/v1/lakes/"+lake.ID+"/invites", nil, &invList); code != http.StatusOK {
		t.Fatalf("list invites: %d", code)
	}
	if len(invList.Invites) != 1 {
		t.Fatalf("want 1 invite, got %d", len(invList.Invites))
	}

	// ---- user B 注册+登录 ----
	emailB := fmt.Sprintf("joiner+%s@ripple.local", uuid.NewString()[:8])
	passwordB := "Test1234!"
	if code := f.do(t, "POST", "/api/v1/auth/register", map[string]string{
		"email": emailB, "password": passwordB, "display_name": "B",
	}, nil); code != http.StatusCreated {
		t.Fatalf("B register: %d", code)
	}
	var loginB struct{ AccessToken string `json:"access_token"` }
	if code := f.do(t, "POST", "/api/v1/auth/login", map[string]string{
		"email": emailB, "password": passwordB,
	}, &loginB); code != http.StatusOK {
		t.Fatalf("B login: %d", code)
	}
	f.tok = loginB.AccessToken

	// B 预览邀请
	var preview struct {
		LakeID string `json:"lake_id"`
		Alive  bool   `json:"alive"`
	}
	if code := f.do(t, "GET", "/api/v1/invites/preview?token="+invite.Token, nil, &preview); code != http.StatusOK {
		t.Fatalf("preview: %d", code)
	}
	if preview.LakeID != lake.ID || !preview.Alive {
		t.Fatalf("bad preview: %+v", preview)
	}

	// B 接受
	var accept struct {
		LakeID        string `json:"lake_id"`
		Role          string `json:"role"`
		AlreadyMember bool   `json:"already_member"`
	}
	if code := f.do(t, "POST", "/api/v1/invites/accept", map[string]string{"token": invite.Token}, &accept); code != http.StatusOK {
		t.Fatalf("accept: %d", code)
	}
	if accept.LakeID != lake.ID || accept.Role != "PASSENGER" || accept.AlreadyMember {
		t.Fatalf("bad accept: %+v", accept)
	}

	// B 可 GET 该 lake（有 membership）
	var lakeGet struct{ ID string `json:"id"` }
	for i := 0; i < 10; i++ {
		if code := f.do(t, "GET", "/api/v1/lakes/"+lake.ID, nil, &lakeGet); code == http.StatusOK && lakeGet.ID == lake.ID {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if lakeGet.ID != lake.ID {
		t.Fatal("B cannot access lake after accept")
	}

	// 另一个用户 C 再用同一个 token（已耗尽）
	emailC := fmt.Sprintf("late+%s@ripple.local", uuid.NewString()[:8])
	if code := f.do(t, "POST", "/api/v1/auth/register", map[string]string{
		"email": emailC, "password": passwordB, "display_name": "C",
	}, nil); code != http.StatusCreated {
		t.Fatalf("C register: %d", code)
	}
	var loginC struct{ AccessToken string `json:"access_token"` }
	if code := f.do(t, "POST", "/api/v1/auth/login", map[string]string{
		"email": emailC, "password": passwordB,
	}, &loginC); code != http.StatusOK {
		t.Fatalf("C login: %d", code)
	}
	f.tok = loginC.AccessToken
	if code := f.do(t, "POST", "/api/v1/invites/accept", map[string]string{"token": invite.Token}, nil); code != http.StatusBadRequest {
		t.Fatalf("C accept exhausted: want 400, got %d", code)
	}

	// ---- A 再签一枚，然后撤销，C 再接受应失败 ----
	f.tok = loginA.AccessToken
	var invite2 struct {
		ID    string `json:"id"`
		Token string `json:"token"`
	}
	if code := f.do(t, "POST", "/api/v1/lakes/"+lake.ID+"/invites", map[string]any{
		"role": "PASSENGER", "max_uses": 1, "ttl_seconds": 3600,
	}, &invite2); code != http.StatusCreated {
		t.Fatalf("create invite2: %d", code)
	}
	if code := f.do(t, "DELETE", "/api/v1/lake-invites/"+invite2.ID, nil, nil); code != http.StatusNoContent {
		t.Fatalf("revoke: %d", code)
	}
	// 幂等撤销
	if code := f.do(t, "DELETE", "/api/v1/lake-invites/"+invite2.ID, nil, nil); code != http.StatusNoContent {
		t.Fatalf("idempotent revoke: %d", code)
	}
	f.tok = loginC.AccessToken
	if code := f.do(t, "POST", "/api/v1/invites/accept", map[string]string{"token": invite2.Token}, nil); code != http.StatusBadRequest {
		t.Fatalf("C accept revoked: want 400, got %d", code)
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

// TestIntegrationNodeRevisionFlow 走通节点编辑与回滚：
//  1. 建湖→建节点（自动记 rev 1 "initial"）
//  2. 更新内容（rev 2）→ 再更新（rev 3）
//  3. 列出 revisions，按时间倒序
//  4. 回滚到 rev 1 → 内容恢复为初始 → 产生 rev 4 reason "rollback to rev 1"
func TestIntegrationNodeRevisionFlow(t *testing.T) {
	f := setup(t)

	email := fmt.Sprintf("rev+%s@ripple.local", uuid.NewString()[:8])
	password := "Test1234!"
	if code := f.do(t, "POST", "/api/v1/auth/register", map[string]string{
		"email": email, "password": password, "display_name": "rev",
	}, nil); code != http.StatusCreated {
		t.Fatalf("register: %d", code)
	}
	var login struct {
		AccessToken string `json:"access_token"`
	}
	if code := f.do(t, "POST", "/api/v1/auth/login", map[string]string{
		"email": email, "password": password,
	}, &login); code != http.StatusOK {
		t.Fatalf("login: %d", code)
	}
	f.tok = login.AccessToken

	var lake struct {
		ID string `json:"id"`
	}
	if code := f.do(t, "POST", "/api/v1/lakes", map[string]any{
		"name": "rev-lake-" + uuid.NewString()[:6], "is_public": false,
	}, &lake); code != http.StatusCreated {
		t.Fatalf("create lake: %d", code)
	}
	// 等 outbox 投到 Neo4j
	for i := 0; i < 30; i++ {
		var l struct {
			ID string `json:"id"`
		}
		if code := f.do(t, "GET", "/api/v1/lakes/"+lake.ID, nil, &l); code == http.StatusOK && l.ID == lake.ID {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	initial := "初始内容 v1"
	var node struct {
		ID string `json:"id"`
	}
	if code := f.do(t, "POST", "/api/v1/nodes", map[string]any{
		"lake_id": lake.ID, "content": initial, "type": "TEXT",
	}, &node); code != http.StatusCreated {
		t.Fatalf("create node: %d", code)
	}

	// 更新到 v2
	if code := f.do(t, "PUT", "/api/v1/nodes/"+node.ID+"/content", map[string]any{
		"content": "内容 v2", "edit_reason": "typo fix",
	}, nil); code != http.StatusOK {
		t.Fatalf("update v2: %d", code)
	}
	// 更新到 v3
	if code := f.do(t, "PUT", "/api/v1/nodes/"+node.ID+"/content", map[string]any{
		"content": "内容 v3", "edit_reason": "rewrite",
	}, nil); code != http.StatusOK {
		t.Fatalf("update v3: %d", code)
	}
	// 同内容更新：幂等不新增 rev
	if code := f.do(t, "PUT", "/api/v1/nodes/"+node.ID+"/content", map[string]any{
		"content": "内容 v3",
	}, nil); code != http.StatusOK {
		t.Fatalf("update same: %d", code)
	}

	// 列表 revisions
	var revList struct {
		Revisions []struct {
			RevNumber  int    `json:"rev_number"`
			Content    string `json:"content"`
			EditReason string `json:"edit_reason"`
		} `json:"revisions"`
	}
	if code := f.do(t, "GET", "/api/v1/nodes/"+node.ID+"/revisions?limit=50", nil, &revList); code != http.StatusOK {
		t.Fatalf("list revisions: %d", code)
	}
	if len(revList.Revisions) != 3 {
		t.Fatalf("want 3 revisions, got %d: %+v", len(revList.Revisions), revList.Revisions)
	}
	// 按 created_at DESC
	if revList.Revisions[0].RevNumber != 3 || revList.Revisions[2].RevNumber != 1 {
		t.Fatalf("rev order wrong: %+v", revList.Revisions)
	}
	if revList.Revisions[2].Content != initial {
		t.Fatalf("rev 1 content want %q, got %q", initial, revList.Revisions[2].Content)
	}

	// 回滚到 rev 1
	if code := f.do(t, "POST", "/api/v1/nodes/"+node.ID+"/rollback", map[string]any{
		"target_rev_number": 1,
	}, nil); code != http.StatusOK {
		t.Fatalf("rollback: %d", code)
	}

	// 列表应有 4 条，最新 content=initial
	if code := f.do(t, "GET", "/api/v1/nodes/"+node.ID+"/revisions?limit=50", nil, &revList); code != http.StatusOK {
		t.Fatalf("list after rollback: %d", code)
	}
	if len(revList.Revisions) != 4 {
		t.Fatalf("after rollback want 4 revs, got %d", len(revList.Revisions))
	}
	if revList.Revisions[0].RevNumber != 4 || revList.Revisions[0].Content != initial {
		t.Fatalf("rev 4 should equal initial: %+v", revList.Revisions[0])
	}

	// 非法 target_rev_number
	if code := f.do(t, "POST", "/api/v1/nodes/"+node.ID+"/rollback", map[string]any{
		"target_rev_number": 999,
	}, nil); code != http.StatusNotFound && code != http.StatusBadRequest {
		t.Fatalf("rollback invalid: want 4xx, got %d", code)
	}
}

// TestIntegrationPresenceFlow 校验在线状态端点：
//  1. 初始空列表
//  2. 调 service 层 Join（因 HTTP 层仅通过 WS 触发 Join）
//  3. GET /presence 返回该 user
//  4. Leave 后列表空
func TestIntegrationPresenceFlow(t *testing.T) {
	f := setup(t)

	email := fmt.Sprintf("pres+%s@ripple.local", uuid.NewString()[:8])
	password := "Test1234!"
	if code := f.do(t, "POST", "/api/v1/auth/register", map[string]string{
		"email": email, "password": password, "display_name": "pres",
	}, nil); code != http.StatusCreated {
		t.Fatalf("register: %d", code)
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
		t.Fatalf("login: %d", code)
	}
	f.tok = login.AccessToken
	userID := login.User.ID

	var lake struct {
		ID string `json:"id"`
	}
	if code := f.do(t, "POST", "/api/v1/lakes", map[string]any{
		"name": "pres-lake-" + uuid.NewString()[:6], "is_public": false,
	}, &lake); code != http.StatusCreated {
		t.Fatalf("create lake: %d", code)
	}
	// 等 outbox
	for i := 0; i < 30; i++ {
		var l struct {
			ID string `json:"id"`
		}
		if code := f.do(t, "GET", "/api/v1/lakes/"+lake.ID, nil, &l); code == http.StatusOK && l.ID == lake.ID {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	// 初始空
	var pr struct {
		Users []string `json:"users"`
	}
	if code := f.do(t, "GET", "/api/v1/lakes/"+lake.ID+"/presence", nil, &pr); code != http.StatusOK {
		t.Fatalf("presence initial: %d", code)
	}
	if len(pr.Users) != 0 {
		t.Fatalf("initial presence want empty, got %v", pr.Users)
	}

	// 不开 WS，直接重新装配一个 presence.Service 来 Join（复用 router 背后的 svc 需通过 WS，
	// 这里新建一个连接同一 Redis 的 svc 来触发 Join，然后 HTTP 查询同一 key）
	cfg, _ := config.Load()
	rds, err := store.NewRedis(context.Background(), cfg)
	if err != nil {
		t.Skipf("presence flow requires redis: %v", err)
	}
	defer func() { _ = rds.Close() }()
	localBroker := realtime.NewMemoryBroker(8)
	defer func() { _ = localBroker.Close() }()
	svc := presence.NewService(rds, localBroker, 0)

	if err := svc.Join(context.Background(), lake.ID, userID); err != nil {
		t.Fatalf("svc.Join: %v", err)
	}
	pr.Users = nil
	if code := f.do(t, "GET", "/api/v1/lakes/"+lake.ID+"/presence", nil, &pr); code != http.StatusOK {
		t.Fatalf("presence after join: %d", code)
	}
	if len(pr.Users) != 1 || pr.Users[0] != userID {
		t.Fatalf("after join want [%s], got %v", userID, pr.Users)
	}

	if err := svc.Leave(context.Background(), lake.ID, userID); err != nil {
		t.Fatalf("svc.Leave: %v", err)
	}
	pr.Users = nil
	if code := f.do(t, "GET", "/api/v1/lakes/"+lake.ID+"/presence", nil, &pr); code != http.StatusOK {
		t.Fatalf("presence after leave: %d", code)
	}
	if len(pr.Users) != 0 {
		t.Fatalf("after leave want empty, got %v", pr.Users)
	}
}

// TestIntegration_WsToken 验证 POST /api/v1/ws_token 端点（P7-D）：
//
//  1. 有效主 token → 200，返回 purpose="ws" 的短期 token（expires_in=300）。
//  2. ws-only token 自我续期 → 403（禁止无限续期）。
//  3. 无 token → 401（AuthMiddleware 拦截）。
func TestIntegration_WsToken(t *testing.T) {
	f := setup(t)

	email := fmt.Sprintf("wstoken+%s@ripple.local", uuid.NewString()[:8])
	password := "Test1234!"
	if code := f.do(t, "POST", "/api/v1/auth/register", map[string]string{
		"email": email, "password": password, "display_name": "wstoken-user",
	}, nil); code != http.StatusCreated {
		t.Fatalf("register: want 201, got %d", code)
	}

	var login struct {
		AccessToken string `json:"access_token"`
	}
	if code := f.do(t, "POST", "/api/v1/auth/login", map[string]string{
		"email": email, "password": password,
	}, &login); code != http.StatusOK {
		t.Fatalf("login: want 200, got %d", code)
	}
	f.tok = login.AccessToken

	// 场景 1：有效主 token → 200 + ws token
	var resp1 struct {
		Token     string `json:"token"`
		ExpiresIn int    `json:"expires_in"`
	}
	if code := f.do(t, "POST", "/api/v1/ws_token", nil, &resp1); code != http.StatusOK {
		t.Fatalf("ws_token with main token: want 200, got %d", code)
	}
	if resp1.Token == "" {
		t.Fatal("ws_token: got empty token")
	}
	if resp1.ExpiresIn != 300 {
		t.Fatalf("ws_token: want expires_in=300, got %d", resp1.ExpiresIn)
	}

	// 验证返回的 token purpose="ws"（使用同一 JWTSigner 解析）
	cfg := f.cfg
	signer := platform.NewJWTSigner(cfg.JWTSecret, cfg.JWTExpiresIn)
	claims, err := signer.Parse(resp1.Token)
	if err != nil {
		t.Fatalf("parse ws token: %v", err)
	}
	if claims.Purpose != "ws" {
		t.Fatalf("ws token purpose: want 'ws', got %q", claims.Purpose)
	}

	// 场景 2：用 ws-only token 续期 → 403
	f.tok = resp1.Token
	if code := f.do(t, "POST", "/api/v1/ws_token", nil, nil); code != http.StatusForbidden {
		t.Fatalf("ws token renewal: want 403, got %d", code)
	}

	// 场景 3：无 token → 401
	f.tok = ""
	if code := f.do(t, "POST", "/api/v1/ws_token", nil, nil); code != http.StatusUnauthorized {
		t.Fatalf("no token: want 401, got %d", code)
	}
}

