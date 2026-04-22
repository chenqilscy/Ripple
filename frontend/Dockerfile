# ---- Build stage ----
FROM node:20-alpine AS builder

WORKDIR /app

# Cache dependencies
COPY package.json package-lock.json* ./
RUN npm ci

COPY . .

# VITE_API_BASE="" makes HTTP API calls use relative paths (nginx proxies /api/)
# VITE_API_HOST is left unset so WebSocket falls back to window.location.hostname:8000
ARG VITE_API_BASE=""
ENV VITE_API_BASE=$VITE_API_BASE

RUN npm run build

# ---- Runtime stage ----
FROM nginx:1.27-alpine

COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80
