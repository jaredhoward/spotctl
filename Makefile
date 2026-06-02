BINARY=spotctl
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

# Default package to test. Override with PKG=spotify or PKG=./spotify.
PKG ?= ./...

# Module-wide coverage by default, or package-local coverage for a specific package.
TESTPKG := $(if $(filter ./...,$(PKG)),$(PKG),$(if $(findstring ./,$(PKG)),$(PKG),./$(PKG)))
COVERPKG := $(TESTPKG)
COVERPROFILE := $(if $(filter ./...,$(TESTPKG)),coverage.out,$(notdir $(TESTPKG)).out)

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

clean:
	rm -f $(BINARY) $(BINARY)-*

.PHONY: build build-ha-green test test-v coverage check-tag clean
