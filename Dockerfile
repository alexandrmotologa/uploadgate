# Build web cockpit
FROM node:22-alpine AS web-builder
WORKDIR /app/web
COPY web/package*.json ./
RUN npm ci || npm install
COPY web/ ./
RUN npm run build

# Build client widget
FROM node:22-alpine AS widget-builder
WORKDIR /app/widget
COPY widget/package*.json ./
RUN npm ci || npm install
COPY widget/ ./
RUN npm run build

# Build Go daemon
FROM golang:1.24-alpine AS go-builder
WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
COPY --from=web-builder /app/web/dist ./web/dist
COPY --from=widget-builder /app/widget/dist ./widget/dist
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /app/uploadgate ./cmd/uploadgate

# Minimal runtime image
FROM alpine:3.21
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app
COPY --from=go-builder /app/uploadgate /usr/local/bin/uploadgate
COPY config.example.yaml /app/config.yaml

EXPOSE 8080
VOLUME ["/app/data", "/app/local_storage"]

ENTRYPOINT ["/usr/local/bin/uploadgate"]
CMD ["serve", "--config", "/app/config.yaml"]
