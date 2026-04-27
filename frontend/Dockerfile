# ---- Build stage ----
FROM node:20-alpine AS builder

WORKDIR /app
ENV NODE_ENV=development
ARG NPM_REGISTRY=https://registry.npmmirror.com

# Cache dependencies
COPY package.json package-lock.json* ./
RUN npm config set registry ${NPM_REGISTRY} \
	&& npm config set replace-registry-host always \
	&& npm config set fetch-retries 5 \
	&& npm config set fetch-retry-mintimeout 20000 \
	&& npm config set fetch-retry-maxtimeout 120000 \
	&& npm ci --include=dev --no-audit --no-fund

COPY . .

# VITE_API_BASE="" makes HTTP API calls use relative paths (nginx proxies /api/).
# VITE_API_HOST is left unset so WebSocket uses the current origin and the same /api proxy.
ARG VITE_API_BASE=""
ENV VITE_API_BASE=$VITE_API_BASE

RUN npm run build

# ---- Runtime stage ----
FROM nginx:1.27-alpine

COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80
