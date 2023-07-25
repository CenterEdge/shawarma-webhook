FROM --platform=$BUILDPLATFORM golang:1.20 as build

# Install modules first for caching
WORKDIR /app
ENV GO111MODULE=on
COPY go.* ./
RUN --mount=type=cache,target=/go/pkg \
    go mod download

# Build the application
ARG VERSION
COPY ./ ./
ARG TARGETOS TARGETARCH
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -ldflags="-s -w -X main.version=${VERSION:-0.0.0}"



# Copy compiled output to a fresh image
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
WORKDIR /etc/shawarma-webhook

COPY --from=build ["/app/shawarma-webhook", "/app/sidecar.yaml", "./"]

# Ensure the tmp folder is available
VOLUME [ "/tmp", "/etc/shawarma-webhook/certs" ]

ENV CERT_FILE=/etc/shawarma-webhook/certs/tls.crt \
    KEY_FILE=/etc/shawarma-webhook/certs/tls.key \
    WEBHOOK_PORT=443 \
    SHAWARMA_IMAGE=centeredge/shawarma:1.1.0 \
    SHAWARMA_SERVICE_ACCT_NAME=shawarma \
    LOG_LEVEL=warn

USER 65532:65532
ENTRYPOINT [ "/etc/shawarma-webhook/shawarma-webhook" ]
CMD []
