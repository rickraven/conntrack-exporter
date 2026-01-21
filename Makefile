ARTIFACT_DIR := .artifacts
VERSION ?= dev

.PHONY: build
build:
	cd src && go build -trimpath -ldflags="-s -w -X main.version=$(VERSION)" -o ../$(ARTIFACT_DIR)/conntrack-exporter ./cmd/conntrack-exporter

.PHONY: tidy
tidy:
	cd src && go mod tidy

.PHONY: artifacts
artifacts:
	mkdir -p $(ARTIFACT_DIR)
	docker build -f docker/Dockerfile.alpine -t conntrack-exporter:alpine .
	docker run --rm -v "$(CURDIR)/$(ARTIFACT_DIR):/out" --entrypoint "" conntrack-exporter:alpine cp /conntrack-exporter /out

.PHONY: docker-image
docker-image:
	docker build -f docker/Dockerfile -t conntrack-exporter:latest .

.PHONY: docker-alpine
docker-alpine:
	docker build -f docker/Dockerfile.alpine -t conntrack-exporter:alpine .

