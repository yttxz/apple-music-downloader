APP_NAME := apple-music-dl
DIST_DIR := dist
MACOS_ARM64_DIR := $(DIST_DIR)/darwin-arm64
GO_CACHE_DIR := $(CURDIR)/.cache/go-build
GO_PATH_DIR := $(CURDIR)/.cache/go
GO_ENV := GOCACHE=$(GO_CACHE_DIR) GOPATH=$(GO_PATH_DIR)

.PHONY: build build-macos-arm64 package-macos-arm64 clean

build:
	@mkdir -p $(DIST_DIR)
	$(GO_ENV) CGO_ENABLED=0 go build -trimpath -o $(DIST_DIR)/$(APP_NAME) .

build-macos-arm64:
	@mkdir -p $(MACOS_ARM64_DIR)
	$(GO_ENV) GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o $(MACOS_ARM64_DIR)/$(APP_NAME) .

package-macos-arm64: build-macos-arm64
	cp agent.js agent-arm64.js README.md $(MACOS_ARM64_DIR)/
	cp config.yaml.example $(MACOS_ARM64_DIR)/config.yaml
	cd $(DIST_DIR) && tar -czf $(APP_NAME)-darwin-arm64.tar.gz darwin-arm64

clean:
	rm -rf $(DIST_DIR) .cache
