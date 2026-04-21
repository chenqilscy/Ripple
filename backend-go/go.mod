module github.com/chenqilscy/ripple/backend-go

go 1.23

// 依赖将在收到老板提供的中间件信息后用 `go mod tidy` 真正引入。
// 本文件仅声明意图与版本基线，不引入任何具体版本号以避免误导。

// 计划引入：
//   github.com/go-chi/chi/v5
//   github.com/jackc/pgx/v5
//   github.com/neo4j/neo4j-go-driver/v5
//   github.com/redis/go-redis/v9
//   nhooyr.io/websocket
//   github.com/golang-jwt/jwt/v5
//   github.com/kelseyhightower/envconfig
//   github.com/rs/zerolog
//   golang.org/x/crypto/bcrypt
//   github.com/google/uuid
//   github.com/danielgtaylor/huma/v2  (OpenAPI 自动生成)
