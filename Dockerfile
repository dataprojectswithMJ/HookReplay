# syntax=docker/dockerfile:1
# HookReplay API — Go. Templates are embedded via //go:embed at build time.
FROM golang:1.25-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/hookreplay-api ./cmd/api

FROM alpine:3.20
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 app
COPY --from=build /out/hookreplay-api /usr/local/bin/hookreplay-api
USER app
EXPOSE 8080
ENTRYPOINT ["hookreplay-api"]
