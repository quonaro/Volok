# syntax=docker/dockerfile:1

# Stage 1: Build Go binary
FROM golang:1.27-alpine AS builder
ENV GOPROXY=https://proxy.golang.org,direct
RUN apk add --no-cache ca-certificates git
WORKDIR /build
COPY go.mod ./
RUN go mod download
COPY . .
ARG VERSION
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} GOARM=${TARGETVARIANT#v} \
    go build -trimpath -ldflags="-s -w -X volok/internal/cli.version=${VERSION}" \
    -o volok ./cmd/volok

# Stage 2: Minimal runtime image
FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /build/volok /volok
VOLUME ["/etc/volok"]
ENTRYPOINT ["/volok", "--file", "/etc/volok/volok.json"]
CMD ["serve"]
