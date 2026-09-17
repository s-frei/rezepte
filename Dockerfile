# Stage 1: build the SPA into service/internal/web/dist
FROM oven/bun:1.4.2 AS frontend
WORKDIR /src
COPY frontend/package.json frontend/bun.lock ./frontend/
RUN cd frontend && bun install --frozen-lockfile
COPY frontend ./frontend
RUN mkdir -p service/internal/web/dist && cd frontend && bun run build

# Stage 2: build the static Go binary
FROM golang:1.27.1 AS backend
WORKDIR /src/service
COPY service/go.mod service/go.sum ./
RUN go mod download
COPY service ./
COPY --from=frontend /src/service/internal/web/dist ./internal/web/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/rezepte ./cmd/rezepte
RUN mkdir -p /out/data && chown 65532:65532 /out/data

# Stage 3: minimal runtime
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=backend /out/rezepte /rezepte
ENV REZEPTE_ADDR=:8080 REZEPTE_DATA_DIR=/data
COPY --from=backend --chown=65532:65532 /out/data /data
VOLUME /data
EXPOSE 8080
ENTRYPOINT ["/rezepte"]
