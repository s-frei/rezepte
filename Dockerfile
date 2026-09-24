# Stage 1: build the SPA into service/internal/web/dist
FROM --platform=$BUILDPLATFORM oven/bun:1.4.2 AS frontend
WORKDIR /src
COPY frontend/package.json frontend/bun.lock ./frontend/
RUN cd frontend && bun install --frozen-lockfile
COPY frontend ./frontend
# The list of languages sits in the Go module so the service can embed it;
# vite.config.ts generates Paraglide's settings from it.
COPY service/internal/i18n/locales.json ./service/internal/i18n/locales.json
# The logo lockups sit outside frontend/, next to the other logo sources;
# the SPA imports them through the $brand alias.
COPY assets/brand/lockups ./assets/brand/lockups
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
# Only labels docker/metadata-action does not also emit. A `--label` on the
# build command beats a LABEL here, and the release workflow passes that
# action's labels, so anything it produces - title, description, url, version,
# revision, created - would be overridden on exactly the images that matter and
# survive only on local ones. Those live in the workflow instead, where both
# paths agree. `documentation` is ours because the action has no equivalent;
# `source` and `licenses` are repeated deliberately, so an image built here or
# in CI still says where it came from and under what terms.
LABEL org.opencontainers.image.source="https://github.com/s-frei/rezepte" \
	org.opencontainers.image.documentation="https://s-frei.github.io/rezepte/" \
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
