# Stage 1: build the SPA into service/internal/web/dist
FROM --platform=$BUILDPLATFORM oven/bun:1.4.2 AS frontend
WORKDIR /src
COPY frontend/package.json frontend/bun.lock ./frontend/
RUN cd frontend && bun install --frozen-lockfile
COPY frontend ./frontend
RUN mkdir -p service/internal/web/dist && cd frontend && bun run build

# Stage 2: build the static Go binary
FROM --platform=$BUILDPLATFORM golang:1.27.1 AS backend
WORKDIR /src/service
COPY service/go.mod service/go.sum ./
RUN go mod download
COPY service ./
COPY --from=frontend /src/service/internal/web/dist ./internal/web/dist
# TARGETARCH comes from BuildKit. Building on $BUILDPLATFORM and cross-compiling
# keeps QEMU out of the multi-platform build: modernc.org/sqlite is pure Go and
# the build is already CGO_ENABLED=0, so the target is two variables. The
# version arrives the same way service/mise.toml's build:only task takes it:
# a REZEPTE_VERSION build arg, defaulting to "dev" when none is given.
ARG TARGETARCH
ARG REZEPTE_VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH \
	go build -trimpath -ldflags="-s -w -X main.version=${REZEPTE_VERSION}" -o /out/rezepte ./cmd/rezepte
RUN mkdir -p /out/data && chown 65532:65532 /out/data

# Stage 3: minimal runtime
FROM gcr.io/distroless/static-debian12:nonroot
# Only the labels that never change. GHCR links a package to its repository
# through `source`, and it has to hold for an image built here or in CI, not
# just for one the release workflow pushed. The per-build labels - version,
# revision, created - come from docker/metadata-action and are not repeated
# here, where they would be a second, stale answer.
LABEL org.opencontainers.image.source="https://github.com/s-frei/rezepte" \
	org.opencontainers.image.url="https://s-frei.github.io/rezepte/" \
	org.opencontainers.image.documentation="https://s-frei.github.io/rezepte/" \
	org.opencontainers.image.title="Rezepte" \
	org.opencontainers.image.description="Self-hosted recipe manager with the web app built in and SQLite storage" \
	org.opencontainers.image.licenses="Apache-2.0"
COPY --from=backend /out/rezepte /rezepte
ENV REZEPTE_ADDR=:8060 REZEPTE_DATA_DIR=/data
COPY --from=backend --chown=65532:65532 /out/data /data
VOLUME /data
EXPOSE 8060
# The binary probes itself: this image has no shell and no curl for a
# HEALTHCHECK to call. --start-period covers the first start, where migrations
# and, with --demo, seeding run before the server answers; failures during it
# do not count against --retries.
HEALTHCHECK --interval=30s --timeout=5s --start-period=30s --retries=3 \
	CMD ["/rezepte", "--healthcheck"]
ENTRYPOINT ["/rezepte"]
