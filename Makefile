.PHONY: tidy fmt lint vet vulncheck verify verify-strict check build dist run clean add-feature add-feature-git codemap-check codemap-sync map ci-init test

GO := go
APP_NAME := fkn
KETUU := ./tools/ketuu
BIN := bin/$(APP_NAME)
DIST := dist
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X main.version=$(VERSION)
CHECKSUM_CMD := $(shell if command -v sha256sum >/dev/null 2>&1; then echo sha256sum; else echo "shasum -a 256"; fi)

tidy:
	$(GO) mod tidy

fmt:
	gofmt -w .

lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not found; skipping lint"; \
	fi

vet:
	$(GO) vet ./...

vulncheck:
	@if command -v govulncheck >/dev/null 2>&1; then \
		govulncheck ./...; \
	else \
		echo "govulncheck not found; skipping vulncheck"; \
	fi

verify:
	@if [ -x "$(KETUU)" ]; then \
		"$(KETUU)" verify; \
	else \
		ketuu verify; \
	fi

verify-strict:
	@if [ -x "$(KETUU)" ]; then \
		"$(KETUU)" verify --strict; \
	else \
		ketuu verify --strict; \
	fi

codemap-check:
	@if [ -x "$(KETUU)" ]; then \
		"$(KETUU)" verify --codemap; \
	else \
		ketuu verify --codemap; \
	fi

codemap-sync:
	@if [ -x "$(KETUU)" ]; then \
		"$(KETUU)" codemap sync; \
	else \
		ketuu codemap sync; \
	fi

map:
	@if [ -x "$(KETUU)" ]; then \
		"$(KETUU)" map; \
	else \
		ketuu map; \
	fi

ci-init:
	@if [ -x "$(KETUU)" ]; then \
		"$(KETUU)" ci init --github; \
	else \
		ketuu ci init --github; \
	fi

check: fmt lint vet verify test vulncheck

test:
	$(GO) test ./...

build: tidy
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/$(APP_NAME)

dist: tidy
	rm -rf $(DIST)
	mkdir -p $(DIST)
	@set -e; \
	for target in darwin/amd64 darwin/arm64 linux/amd64 linux/arm64 windows/amd64 windows/arm64; do \
		goos=$${target%/*}; \
		goarch=$${target#*/}; \
		name="$(APP_NAME)_$(VERSION)_$${goos}_$${goarch}"; \
		ext=""; \
		if [ "$${goos}" = "windows" ]; then ext=".exe"; fi; \
		outdir="$(DIST)/$$name"; \
		mkdir -p "$$outdir"; \
		GOOS=$${goos} GOARCH=$${goarch} $(GO) build -ldflags "$(LDFLAGS)" -o "$$outdir/$(APP_NAME)$$ext" ./cmd/$(APP_NAME); \
		if [ "$${goos}" = "windows" ]; then \
			(cd "$(DIST)" && zip -qr "$$name.zip" "$$name"); \
		else \
			(cd "$(DIST)" && tar -czf "$$name.tar.gz" "$$name"); \
		fi; \
		rm -rf "$$outdir"; \
	done
	@cd $(DIST) && $(CHECKSUM_CMD) *.tar.gz *.zip > checksums.txt

run: build
	$(BIN)

add-feature:
	@if [ -z "$(FEATURE)" ]; then \
		echo "FEATURE is required (example: make add-feature FEATURE=export-report)"; \
		exit 1; \
	fi
	@if [ -x "$(KETUU)" ]; then \
		"$(KETUU)" add-feature "$(FEATURE)" --transport cli; \
	else \
		ketuu add-feature "$(FEATURE)" --transport cli; \
	fi

add-feature-git:
	@if [ -z "$(FEATURE)" ]; then \
		echo "FEATURE is required (example: make add-feature-git FEATURE=export-report)"; \
		exit 1; \
	fi
	@if [ -x "$(KETUU)" ]; then \
		"$(KETUU)" add-feature "$(FEATURE)" --transport cli --branch --commit; \
	else \
		ketuu add-feature "$(FEATURE)" --transport cli --branch --commit; \
	fi

clean:
	rm -rf bin dist tmp
