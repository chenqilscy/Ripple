# === builder ===
FROM golang:1.23-alpine AS builder
ARG GOPROXY=https://goproxy.cn,direct
ENV GOPROXY=$GOPROXY GOTOOLCHAIN=local
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/ripple-go ./cmd/server

# === runtime ===
FROM golang:1.23-alpine
WORKDIR /app
RUN adduser -D -u 10001 ripple
COPY --from=builder /out/ripple-go /app/ripple-go
EXPOSE 8000
USER ripple
ENTRYPOINT ["/app/ripple-go"]
