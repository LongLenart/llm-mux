# Stage 1 Build
FROM golang:1.25-alpine AS builder

WORKDIR /build

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
ARG COMMIT=none
ARG BUILD_DATE=unknown

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -buildvcs=false \
    -ldflags="-s -w -X main.Version=${VERSION} -X main.Commit=${COMMIT} -X main.BuildDate=${BUILD_DATE}" \
    -o llm-mux \
    ./cmd/server

# Stage 2 Runtime
FROM alpine:3.23

RUN apk add --no-cache \
    tzdata \
    ca-certificates \
    bash \
    curl

RUN addgroup -g 1000 llm-mux \
    && adduser -D -u 1000 -G llm-mux llm-mux \
    && mkdir -p /llm-mux \
    && chown -R llm-mux:llm-mux /llm-mux

WORKDIR /llm-mux

COPY --from=builder /build/llm-mux /llm-mux/llm-mux
RUN chmod +x /llm-mux/llm-mux

USER llm-mux

USER llm-mux

ENV TZ=UTC
ENV PORT=8317
ENV HOST=0.0.0.0
ENV SHELL=/bin/bash

EXPOSE 8317

CMD ["/llm-mux/llm-mux"]
