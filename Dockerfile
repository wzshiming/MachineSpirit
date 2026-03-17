ARG ALPINE_VERSION=3.23
ARG GOLANG_VERSION=1.25

ARG IMAGE_PREFIX=docker.io/
ARG GOPROXY=https://proxy.golang.org,direct

##########################################

FROM ${IMAGE_PREFIX}library/golang:${GOLANG_VERSION}-alpine${ALPINE_VERSION} AS builder

WORKDIR /app

ARG GOPROXY
ENV GOPROXY=${GOPROXY}
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=bind,source=./go.mod,target=/app/go.mod \
    --mount=type=bind,source=./go.sum,target=/app/go.sum \
    go mod download

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 go build -o /ms ./cmd/ms

##########################################

FROM ${IMAGE_PREFIX}library/alpine:${ALPINE_VERSION} AS ms

RUN --mount=type=cache,target=/var/cache/apk \
    apk add ca-certificates bash curl git python3 py3-pip nodejs npm && \
    update-ca-certificates

COPY --from=builder /ms /usr/local/bin/ms

ENTRYPOINT ["/usr/local/bin/ms"]
