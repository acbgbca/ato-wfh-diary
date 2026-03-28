# ── Stage 1: Build ────────────────────────────────────────────────────────────
FROM golang:1.26-alpine@sha256:2389ebfa5b7f43eeafbd6be0c3700cc46690ef842ad962f6c5bd6be49ed82039 AS builder

WORKDIR /src

# Download dependencies first so this layer is cached when only source changes.
COPY backend/go.mod backend/go.sum ./
RUN go mod download

# Copy full backend source and build a fully static binary.
# modernc/sqlite is pure Go — CGO_ENABLED=0 is all that is needed.
COPY backend/ ./

# Copy the top-level frontend source into the location the Go embed expects.
# This mirrors what `make copy-frontend` does for local builds.
COPY frontend/ ./frontend/

# VERSION is injected by the build system (e.g. the Git tag v1.2.3).
# It defaults to "dev" for local docker build invocations without --build-arg.
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /out/server \
    ./cmd/server

# Create a non-root user/group for the runtime image.
# A placeholder file in /out/data ensures the directory survives the COPY into
# scratch and that Docker initialises the named volume with the correct UID/GID
# on first use.
RUN addgroup -S -g 1001 app \
 && adduser  -S -u 1001 -G app app \
 && mkdir -p /out/data \
 && touch    /out/data/.keep \
 && chown -R 1001:1001 /out/data

# ── Stage 2: Runtime ──────────────────────────────────────────────────────────
FROM scratch

# Bring in user/group databases so the USER directive resolves by name.
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /etc/group  /etc/group

# Application binary.
COPY --from=builder /out/server /server

# Data directory pre-created with the correct ownership.  When docker compose
# mounts a named volume here for the first time it copies this directory
# (including ownership) from the image, so the app can write immediately
# without any host-side permission changes.
COPY --from=builder --chown=1001:1001 /out/data /data

USER app

EXPOSE 8080

ENV DB_PATH=/data/wfh.db
ENV FORWARD_AUTH_HEADER=X-Forwarded-User

ENTRYPOINT ["/server"]
