FROM golang:1.25-alpine AS builder

WORKDIR /build

RUN apk add --no-cache git ca-certificates tzdata

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o subkit cmd/server/main.go && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o update-rules cmd/update-rules/main.go

FROM alpine:latest

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata && \
    mkdir -p /app/config/rules /app/config/prompts /app/web/static

COPY --from=builder /build/subkit /app/
COPY --from=builder /build/update-rules /app/
COPY --from=builder /build/web/static /app/web/static
COPY --from=builder /build/config /app/config

ENV PORT=8080 \
    LLM_BASE_URL=https://api.openai.com/v1 \
    LLM_MODEL=gpt-5-mini \
    LLM_TIMEOUT=120s

EXPOSE 8080

CMD ["/app/subkit"]
