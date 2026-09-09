# Multi-stage build for the Partner Portal (Publisher App) React + Vite frontend.
FROM public.ecr.aws/docker/library/node:22-slim AS builder

WORKDIR /app

# Cache npm install on its own layer.
COPY package.json ./
ENV NPM_CONFIG_FETCH_RETRIES=5 \
    NPM_CONFIG_FETCH_RETRY_MINTIMEOUT=20000 \
    NPM_CONFIG_FETCH_RETRY_MAXTIMEOUT=120000 \
    NPM_CONFIG_REGISTRY=https://registry.yarnpkg.com/ \
    NODE_OPTIONS="--dns-result-order=ipv4first"
RUN npm install --no-audit --no-fund

COPY . .

# Vite build
ENV NODE_ENV=production
ENV VITE_API_BASE_URL=/api

RUN npm run build

# ──────────────────────────────────────────────────────────────────────────
FROM public.ecr.aws/docker/library/nginx:1.27-alpine

COPY --from=builder /app/dist /usr/share/nginx/html
COPY docker/nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80
