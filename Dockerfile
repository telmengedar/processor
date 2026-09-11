# syntax=docker/dockerfile:1

# Processor has zero external Go dependencies (go.mod declares none), so this build stage
# downloads nothing and needs no credential — it compiles the standard-library-only tree
# straight into a static binary.
FROM golang:1.27 AS build

WORKDIR /src
COPY . .

ARG TARGETARCH
ENV CGO_ENABLED=0 \
    GOOS=linux

RUN GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w" -o /out/processor ./cmd/processor

# The build stage still has a shell and coreutils, which the final distroless stage does
# not — so the workspace directory is created and chowned to the final image's non-root
# uid here, then copied across with that ownership already correct (see the VOLUME note
# below for exactly what this does and does not fix).
RUN mkdir -p /out/workspace && chown 65532:65532 /out/workspace

# distroless/static carries CA certificates and /etc/nsswitch.conf (both needed for the
# outbound HTTPS calls to the graph and the model endpoint) but no shell and no package
# manager — a smaller surface for a binary that never needs to exec anything.  The
# ":nonroot" tag runs as uid/gid 65532 rather than root.
FROM gcr.io/distroless/static-debian12:nonroot AS final

# The service's own default listen address is 127.0.0.1:8080 (see internal/boot/config.go
# and README's configuration table) — correct for a bare-metal run, and silently
# unreachable from outside a container, since a loopback bind never sees traffic arriving
# on a published port. This override is the one thing that has to differ between "run on
# the host" and "run in a container"; it is not a secret, and it is still overridable with
# `-e PROCESSOR_HTTP_ADDR=...`.
ENV PROCESSOR_HTTP_ADDR=0.0.0.0:8080

EXPOSE 8080

COPY --from=build /out/processor /processor
COPY --from=build --chown=65532:65532 /out/workspace /data/workspace

# Declaring the mount point seeds a *fresh* named volume from this directory's content and
# ownership on first use, but not one that already exists — see README's Container section
# for what that means and the one-time fix for an already-root-owned volume.
VOLUME ["/data/workspace"]

ENTRYPOINT ["/processor"]
