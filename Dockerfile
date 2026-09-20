# Stage 0: the build context as Docker sees it, so `mise run docker:context`
# can list what .dockerignore actually lets through. Nothing depends on this
# stage; a normal build never selects it.
FROM scratch AS build-context
COPY . /

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
# the build is already CGO_ENABLED=0, so the target is two variables.
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH \
	go build -trimpath -ldflags="-s -w" -o /out/rezepte ./cmd/rezepte
RUN mkdir -p /out/data && chown 65532:65532 /out/data

# Stage 3: minimal runtime
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=backend /out/rezepte /rezepte
ENV REZEPTE_ADDR=:8060 REZEPTE_DATA_DIR=/data
COPY --from=backend --chown=65532:65532 /out/data /data
VOLUME /data
EXPOSE 8060
ENTRYPOINT ["/rezepte"]
