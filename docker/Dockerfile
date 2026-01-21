FROM --platform=$BUILDPLATFORM golang:1.25 AS build

WORKDIR /work
COPY src/go.mod src/go.sum ./src/
RUN cd src && go mod download
COPY src ./src

ARG TARGETOS=linux
ARG TARGETARCH=amd64
ARG VERSION=dev

# Build a static binary for the target platform.
RUN set -eu; \
    cd src; \
    CGO_ENABLED=0 GOOS="${TARGETOS}" GOARCH="${TARGETARCH}" \
      go build -trimpath -buildvcs=false \
        -ldflags="-s -w -X main.version=${VERSION}" \
        -o /conntrack-exporter ./cmd/conntrack-exporter


FROM scratch

COPY --from=build /conntrack-exporter /conntrack-exporter

EXPOSE 9095

ENTRYPOINT ["/conntrack-exporter"]

