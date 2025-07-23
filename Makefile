APP_NAME := nuke
VERSION  := $(shell git describe --tags --always --dirty)
DIST     := dist

# GOOS/GOARCH combos
TARGETS := \
	"linux amd64" \
	"darwin amd64" \
	"darwin arm64"

all: release

clean:
	rm -rf $(DIST)

$(DIST):
	mkdir -p $(DIST)

define build_target
	GOOS=$(1) GOARCH=$(2) go build -ldflags="-s -w -X main.version=$(VERSION)" -o $(DIST)/$(APP_NAME)-$(1)-$(2) main.go
	tar -C $(DIST) -czf $(DIST)/$(APP_NAME)-$(1)-$(2).tar.gz $(APP_NAME)-$(1)-$(2)
endef

release: clean $(DIST)
	@echo "Building release version: $(VERSION)"
	@$(foreach t,$(TARGETS),\
		$(eval os := $(word 1, $(t)))\
		$(eval arch := $(word 2, $(t)))\
		$(call build_target,$(os),$(arch))\
	)

.PHONY: all clean release
