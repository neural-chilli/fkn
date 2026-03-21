.PHONY: tidy fmt lint vet vulncheck verify verify-strict check build run clean add-feature add-feature-git codemap-check codemap-sync map ci-init test

GO := go
APP_NAME := fkn
KETUU := ./tools/ketuu
BIN := bin/$(APP_NAME)

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
	$(GO) build -o $(BIN) ./cmd/$(APP_NAME)

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
	rm -rf bin tmp
