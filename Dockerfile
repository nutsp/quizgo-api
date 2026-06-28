# Cross-compile on the native builder arch (avoids Go crashes under QEMU).
# Works with: docker buildx build --platform linux/amd64 --push .
FROM --platform=$BUILDPLATFORM golang:1.24-bookworm AS builder

ARG TARGETOS=linux
ARG TARGETARCH

WORKDIR /app

RUN apt-get update \
  && apt-get install -y --no-install-recommends git ca-certificates \
  && rm -rf /var/lib/apt/lists/*

ENV CGO_ENABLED=0 \
    GOOS=${TARGETOS} \
    GOARCH=${TARGETARCH} \
    GOTELEMETRY=off \
    GOPROXY=https://proxy.golang.org,direct

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod,id=quizgo-gomod-${TARGETARCH} \
    go mod download

COPY . .

RUN --mount=type=cache,target=/go/pkg/mod,id=quizgo-gomod-${TARGETARCH} \
    --mount=type=cache,target=/root/.cache/go-build,id=quizgo-gobuild-${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /virtual-exam-api ./cmd/server

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /virtual-exam-api .

EXPOSE 8080

CMD ["./virtual-exam-api"]
