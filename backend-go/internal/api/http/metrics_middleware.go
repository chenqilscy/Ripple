package httpapi

import (
	"net/http"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/metrics"
)

// metricsMiddleware 统计每个请求的总数 + 耗时分布。
//
// 故意不按 path / status 维度分桶——避免 cardinality 爆炸（chi 的 routePattern 在请求结束前不可拿）。
// 后续如需更细维度，建议接入 chi.RouteContext 在路由匹配后再 wrap。
func metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 跳过 /metrics 自身 + /healthz（健康检查频率高、无业务意义）
		if r.URL.Path == "/metrics" || r.URL.Path == "/healthz" {
			next.ServeHTTP(w, r)
			return
		}
		start := time.Now()
		next.ServeHTTP(w, r)
		metrics.HTTPRequests.Inc()
		metrics.HTTPDuration.Observe(float64(time.Since(start).Milliseconds()))
	})
}
