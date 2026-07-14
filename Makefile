BINARY=spotctl
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

# Default package to test. Override with PKG=spotify or PKG=./spotify.
PKG ?= ./...

# Module-wide coverage by default, or package-local coverage for a specific package.
TESTPKG := $(if $(filter ./...,$(PKG)),$(PKG),$(if $(findstring ./,$(PKG)),$(PKG),./$(PKG)))
COVERPKG := $(TESTPKG)
COVERPROFILE := $(if $(filter ./...,$(TESTPKG)),coverage.out,$(notdir $(TESTPKG)).out)

# Coverage floor enforced in CI (ci.yml); keep these in sync.
COVERAGE_THRESHOLD ?= 85

check-tag:
	@git describe --tags --exact-match 2>/dev/null || \
		(echo "Error: commit must be tagged before release build. Run: git tag v1.x.x" && exit 1)

build:
	go build -ldflags "-X main.version=$(VERSION)" -o $(BINARY) .

build-ha-green: check-tag
	GOOS=linux GOARCH=arm64 go build -ldflags "-X main.version=$(VERSION)" -o $(BINARY)-linux-arm64 .

test:
	go test $(TESTPKG) -coverpkg=$(COVERPKG) -coverprofile=$(COVERPROFILE)

coverage: test
	go tool cover -html=$(COVERPROFILE)

# Full local mirror of ci.yml — run before opening a PR. The pre-commit
# hook (.githooks/pre-commit) only runs the fast subset (vet/build/test)
# on every commit; this adds the coverage gate and govulncheck.
check: test
	go vet ./...
	go build ./...
	@total=$$(go tool cover -func=$(COVERPROFILE) | grep '^total:' | awk '{print $$3}' | tr -d '%'); \
	echo "total coverage: $${total}%"; \
	awk -v t="$$total" -v th="$(COVERAGE_THRESHOLD)" \
		'BEGIN { exit (t+0 < th+0) }' \
		|| { echo "coverage $${total}% is below the $(COVERAGE_THRESHOLD)% threshold"; exit 1; }
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...

clean:
	rm -f $(BINARY) $(BINARY)-*

.PHONY: build build-ha-green test test-v coverage check check-tag clean
