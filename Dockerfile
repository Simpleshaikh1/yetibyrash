# The whole shop as one container: the Go server serves the API, the shop at / and the
# admin at /admin/. Render builds this file on every push (see render.yaml).
#
# Try it locally:  docker build -t yetibyrash . && docker run --env-file backend/.env.production -p 10000:10000 yetibyrash

# 1. Build the shop and admin (React + Vite).
FROM node:22-alpine AS web
WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json ./
COPY frontend/ui/package.json ui/
COPY frontend/shop/package.json shop/
COPY frontend/admin/package.json admin/
RUN npm ci --no-audit --no-fund
COPY frontend/ ./
RUN npm run build

# 2. Build the Go server and the create-owner command.
FROM golang:1.27-alpine AS api
WORKDIR /src/backend
ENV CGO_ENABLED=0 GOWORK=off
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
ARG VERSION=dev
RUN go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/yetibyrash ./cmd/yetibyrash \
 && go build -trimpath -ldflags "-s -w" -o /out/createstaff ./cmd/createstaff

# 3. A small image with just what runs. A shell is kept so the owner account can be
#    created from Render's Shell tab: ./createstaff -email ... -role owner
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata \
 && adduser -D -H -u 10001 shop \
 && mkdir -p /app/var/media && chown -R shop /app/var # only used with MEDIA_STORAGE=local
WORKDIR /app
COPY --from=api /out/ /app/
COPY --from=web /src/frontend/shop/dist /app/web/shop
COPY --from=web /src/frontend/admin/dist /app/web/admin
ENV APP_ENV=production \
    WEB_DIR=/app/web \
    PORT=10000 \
    TZ=Africa/Lagos
USER shop
EXPOSE 10000
CMD ["/app/yetibyrash"]
