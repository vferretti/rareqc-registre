FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
RUN go install github.com/swaggo/swag/cmd/swag@v1.16.6
COPY . .
RUN swag init -g cmd/api/main.go --parseDependency --parseInternal
# -trimpath/-s -w: no build paths or symbol tables in the shipped binary.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o bin/main ./cmd/api/
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o bin/seed ./scripts/seed/

# Runtime images: pinned base, CA bundle for Keycloak over TLS, non-root.
FROM alpine:3.22 AS production
RUN apk add --no-cache ca-certificates && adduser -D -H -u 10001 app
COPY --from=builder /app/bin/main /main
COPY --from=builder /app/migrations /migrations
USER app
EXPOSE 8080
ENTRYPOINT ["/main"]

FROM alpine:3.22 AS seed
RUN apk add --no-cache ca-certificates && adduser -D -H -u 10001 app
COPY --from=builder /app/bin/seed /seed
COPY --from=builder /app/scripts/seed/Consentement_RareQc.pdf /data/Consentement_RareQc.pdf
COPY --from=builder /app/scripts/seed/Consentement_RQDM.pdf /data/Consentement_RQDM.pdf
USER app
ENTRYPOINT ["/seed"]
