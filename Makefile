BINARY=spotctl
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

build:
	go build -ldflags "-X main.version=$(VERSION)" -o $(BINARY) .

build-ha-green: check-tag
	GOOS=linux GOARCH=arm64 go build -ldflags "-X main.version=$(VERSION)" -o $(BINARY)-linux-arm64 .

check-tag:
	@git describe --tags --exact-match 2>/dev/null || \
		(echo "Error: commit must be tagged before release build. Run: git tag v1.x.x" && exit 1)

clean:
	rm -f $(BINARY) $(BINARY)-*

.PHONY: build build-ha-green check-tag clean