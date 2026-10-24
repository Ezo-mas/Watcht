# syntax=docker/dockerfile:1

# watcht container image: the frontend is built with Node, embedded into a
# static Go binary, and shipped on distroless as a non-root user. Base images
# are pinned by digest; update tag and digest together.

# --- Frontend build (runs on the build machine's architecture) ---
FROM --platform=$BUILDPLATFORM node:24.22.1-trixie-slim@sha256:43b9ba1142b8dcf33fa8f922ca1be85714adc444e0586c7cf7726daaeabc1bea AS web
ENV COREPACK_ENABLE_DOWNLOAD_PROMPT=0
RUN corepack enable
WORKDIR /src/web
COPY web/package.json web/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY web/ ./
RUN pnpm build

# --- Go build (cross-compiles for the target architecture) ---
FROM --platform=$BUILDPLATFORM golang:1.27.3-trixie@sha256:fbb7ae40b62ff8afe31662df570d39b4f5052ac32b90dae8e60d7657a20a74f4 AS server
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY --from=web /src/web/build ./internal/webui/dist
ARG TARGETOS TARGETARCH
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/watcht ./cmd/watcht

# --- Runtime ---
FROM gcr.io/distroless/static-debian13:nonroot@sha256:18f4cc26c9ffa7f20a359298be7deadac5c607c8e03a02b8119f1a5fc2f82c99
COPY --from=server /out/watcht /usr/local/bin/watcht
ENV WATCHT_LISTEN=:8080
EXPOSE 8080
# distroless's nonroot user (UID/GID 65532). Override with `user:` in compose
# to match the owner of a mounted data directory.
USER nonroot:nonroot
# distroless has no shell or curl, so the binary checks itself.
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --start-interval=2s --retries=3 \
    CMD ["/usr/local/bin/watcht", "healthcheck"]
ENTRYPOINT ["/usr/local/bin/watcht"]
CMD ["serve"]
