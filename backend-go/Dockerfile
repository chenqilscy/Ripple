# syntax=docker/dockerfile:1.7

# === builder ===
FROM golang:1.23-alpine AS builder
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/ripple-go ./cmd/server

# === runtime ===
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=builder /out/ripple-go /app/ripple-go
EXPOSE 8000
USER nonroot:nonroot
ENTRYPOINT ["/app/ripple-go"]
