FROM --platform=$BUILDPLATFORM golang:1.25 AS build

WORKDIR /work
COPY src/go.mod src/go.sum ./src/
RUN cd src && go mod download
COPY src ./src

ARG TARGETOS=linux
ARG TARGETARCH=amd64
ARG TARGETVARIANT
ARG VERSION=dev

# Build a static binary for the target platform.
# Note: GOARM is only relevant for GOARCH=arm (e.g. linux/arm/v7).
RUN set -eu; \
    cd src; \
    GOARM=""; \
    if [ "${TARGETARCH}" = "arm" ]; then \
      # TARGETVARIANT is like "v7" for linux/arm/v7.
      GOARM="${TARGETVARIANT#v}"; \
    fi; \
    CGO_ENABLED=0 GOOS="${TARGETOS}" GOARCH="${TARGETARCH}" ${GOARM:+GOARM="${GOARM}"} \
      go build -trimpath -buildvcs=false \
        -ldflags="-s -w -X main.version=${VERSION}" \
        -o /conntrack-exporter ./cmd/conntrack-exporter


FROM scratch

COPY --from=build /conntrack-exporter /conntrack-exporter

EXPOSE 9100

ENTRYPOINT ["/conntrack-exporter"]

