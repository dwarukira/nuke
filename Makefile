APP_NAME = nuke
VERSION  = $(shell git describe --tags --always --dirty)
DATE     = $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
COMMIT   = $(shell git rev-parse --short HEAD)

BUILD_FLAGS = -ldflags "-s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"

.PHONY: build release

build:
	@echo "🔧 Building $(APP_NAME)..."
	go build $(BUILD_FLAGS) -o $(APP_NAME) ./main.go

release:
	@echo "📦 Building release version: $(VERSION)"
	@rm -rf dist
	@mkdir -p dist/$(APP_NAME)_darwin_amd64 dist/$(APP_NAME)_darwin_arm64

	GOOS=darwin GOARCH=amd64  go build $(BUILD_FLAGS) -o dist/$(APP_NAME)_darwin_amd64/$(APP_NAME) ./main.go
	GOOS=darwin GOARCH=arm64  go build $(BUILD_FLAGS) -o dist/$(APP_NAME)_darwin_arm64/$(APP_NAME) ./main.go

	@echo "✅ Binaries built:"
	@du -sh dist/*
