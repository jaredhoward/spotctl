BINARY=spotctl
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

# Space-separated for 'go test', comma-separated for -coverpkg.
# The root main package is excluded from both: it has no test files and
# causes a "bad file descriptor" coverage error when included in -coverpkg=./...
TESTPKGS  = ./cmd/... ./config/... ./sets/... ./spotify/...
COVERPKGS = ./cmd/...,./config/...,./sets/...,./spotify/...

check-tag:
	@git describe --tags --exact-match 2>/dev/null || \
		(echo "Error: commit must be tagged before release build. Run: git tag v1.x.x" && exit 1)

build:
	go build -ldflags "-X main.version=$(VERSION)" -o $(BINARY) .

build-ha-green: check-tag
	GOOS=linux GOARCH=arm64 go build -ldflags "-X main.version=$(VERSION)" -o $(BINARY)-linux-arm64 .

test:
	go test $(TESTPKGS) -coverpkg=$(COVERPKGS) -coverprofile=coverage.out

coverage: test
	go tool cover -html=coverage.out

clean:
	rm -f $(BINARY) $(BINARY)-*

.PHONY: build build-ha-green test test-v coverage check-tag clean
