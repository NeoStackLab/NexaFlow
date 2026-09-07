FROM golang:1.26.5-alpine3.24 AS builder

WORKDIR /src
RUN apk add --no-cache ca-certificates git

COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /out/nexaflow-api ./cmd/server

FROM alpine:3.24 AS runtime

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S -g 10001 nexaflow \
    && adduser -S -D -u 10001 -G nexaflow -h /app nexaflow

WORKDIR /app
COPY --from=builder /out/nexaflow-api /app/nexaflow-api
COPY --chown=nexaflow:nexaflow configs/config.production.yaml /app/configs/config.production.yaml
RUN mkdir -p /app/data && chown nexaflow:nexaflow /app/data

USER 10001:10001
EXPOSE 8080

ENTRYPOINT ["/app/nexaflow-api"]
