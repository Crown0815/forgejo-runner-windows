DIST := dist
EXECUTABLE := forgejo-runner
GOFMT ?= gofumpt -l
DIST := dist
DIST_DIRS := $(DIST)/binaries $(DIST)/release
GO ?= go
SHASUM ?= shasum -a 256
HAS_GO = $(shell hash $(GO) > /dev/null 2>&1 && echo "GO" || echo "NOGO" )
XGO_PACKAGE ?= src.techknowlogick.com/xgo@latest
XGO_VERSION := go-1.18.x
GXZ_PAGAGE ?= github.com/ulikunitz/xz/cmd/gxz@v0.5.10

LINUX_ARCHS ?= linux/amd64,linux/arm64
DARWIN_ARCHS ?= darwin-12/amd64,darwin-12/arm64
WINDOWS_ARCHS ?= windows/amd64
GO_FMT_FILES := $(shell find . -type f -name "*.go" ! -name "generated.*")
GOFILES := $(shell find . -type f -name "*.go" -o -name "go.mod" ! -name "generated.*")

DOCKER_IMAGE ?= gitea/act_runner
DOCKER_TAG ?= nightly
DOCKER_REF := $(DOCKER_IMAGE):$(DOCKER_TAG)
DOCKER_ROOTLESS_REF := $(DOCKER_IMAGE):$(DOCKER_TAG)-dind-rootless

EXTLDFLAGS = -extldflags "-static" $(null)

ifeq ($(HAS_GO), GO)
	GOPATH ?= $(shell $(GO) env GOPATH)
	export PATH := $(GOPATH)/bin:$(PATH)

	CGO_EXTRA_CFLAGS := -DSQLITE_MAX_VARIABLE_NUMBER=32766
	CGO_CFLAGS ?= $(shell $(GO) env CGO_CFLAGS) $(CGO_EXTRA_CFLAGS)
endif

ifeq ($(OS), Windows_NT)
	GOFLAGS := -v -buildmode=exe
	EXECUTABLE ?= $(EXECUTABLE).exe
else ifeq ($(OS), Windows)
	GOFLAGS := -v -buildmode=exe
	EXECUTABLE ?= $(EXECUTABLE).exe
else
	GOFLAGS := -v
	EXECUTABLE ?= $(EXECUTABLE)
endif

STORED_VERSION_FILE := VERSION

ifneq ($(DRONE_TAG),)
	VERSION ?= $(subst v,,$(DRONE_TAG))
	RELASE_VERSION ?= $(VERSION)
else
	ifneq ($(DRONE_BRANCH),)
		VERSION ?= $(subst release/v,,$(DRONE_BRANCH))
	else
		VERSION ?= main
	endif

	STORED_VERSION=$(shell cat $(STORED_VERSION_FILE) 2>/dev/null)
	ifneq ($(STORED_VERSION),)
		RELASE_VERSION ?= $(STORED_VERSION)
	else
		RELASE_VERSION ?= $(shell git describe --tags --always | sed 's/-/+/' | sed 's/^v//')
	endif
endif

GO_PACKAGES_TO_VET ?= $(filter-out gitea.com/gitea/act_runner/internal/pkg/client/mocks,$(shell $(GO) list ./...))


TAGS ?=
LDFLAGS ?= -X "gitea.com/gitea/act_runner/internal/pkg/ver.version=v$(RELASE_VERSION)"

all: build

fmt:
	@hash gofumpt > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		$(GO) install mvdan.cc/gofumpt@latest; \
	fi
	$(GOFMT) -w $(GO_FMT_FILES)

.PHONY: go-check
go-check:
	$(eval MIN_GO_VERSION_STR := $(shell grep -Eo '^go\s+[0-9]+\.[0-9]+' go.mod | cut -d' ' -f2))
	$(eval MIN_GO_VERSION := $(shell printf "%03d%03d" $(shell echo '$(MIN_GO_VERSION_STR)' | tr '.' ' ')))
	$(eval GO_VERSION := $(shell printf "%03d%03d" $(shell $(GO) version | grep -Eo '[0-9]+\.[0-9]+' | tr '.' ' ');))
	@if [ "$(GO_VERSION)" -lt "$(MIN_GO_VERSION)" ]; then \
		echo "Act Runner requires Go $(MIN_GO_VERSION_STR) or greater to build. You can get it at https://go.dev/dl/"; \
		exit 1; \
	fi

.PHONY: fmt-check
fmt-check:
	@hash gofumpt > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		$(GO) install mvdan.cc/gofumpt@latest; \
	fi
	@diff=$$($(GOFMT) -d $(GO_FMT_FILES)); \
	if [ -n "$$diff" ]; then \
		echo "Please run 'make fmt' and commit the result:"; \
		echo "$${diff}"; \
		exit 1; \
	fi;

test: fmt-check
	@$(GO) test -v -cover -coverprofile coverage.txt ./... && echo "\n==>\033[32m Ok\033[m\n" || exit 1

.PHONY: vet
vet:
	@echo "Running go vet..."
	@$(GO) vet $(GO_PACKAGES_TO_VET)

install: $(GOFILES)
	$(GO) install -v -tags '$(TAGS)' -ldflags '$(EXTLDFLAGS)-s -w $(LDFLAGS)'

build: go-check $(EXECUTABLE)

$(EXECUTABLE): $(GOFILES)
	$(GO) build -v -tags 'netgo osusergo $(TAGS)' -ldflags '$(EXTLDFLAGS)-s -w $(LDFLAGS)' -o $@

.PHONY: deps-backend
deps-backend:
	$(GO) mod download
	$(GO) install $(GXZ_PAGAGE)
	$(GO) install $(XGO_PACKAGE)

.PHONY: release
release: release-windows release-linux release-darwin release-copy release-compress release-check

$(DIST_DIRS):
	mkdir -p $(DIST_DIRS)

.PHONY: release-windows
release-windows: | $(DIST_DIRS)
	CGO_CFLAGS="$(CGO_CFLAGS)" $(GO) run $(XGO_PACKAGE) -go $(XGO_VERSION) -buildmode exe -dest $(DIST)/binaries -tags 'netgo osusergo $(TAGS)' -ldflags '-linkmode external -extldflags "-static" $(LDFLAGS)' -targets '$(WINDOWS_ARCHS)' -out $(EXECUTABLE)-$(VERSION) .
ifeq ($(CI),true)
	cp -r /build/* $(DIST)/binaries/
endif

.PHONY: release-linux
release-linux: | $(DIST_DIRS)
	CGO_CFLAGS="$(CGO_CFLAGS)" $(GO) run $(XGO_PACKAGE) -go $(XGO_VERSION) -dest $(DIST)/binaries -tags 'netgo osusergo $(TAGS)' -ldflags '-linkmode external -extldflags "-static" $(LDFLAGS)' -targets '$(LINUX_ARCHS)' -out $(EXECUTABLE)-$(VERSION) .
ifeq ($(CI),true)
	cp -r /build/* $(DIST)/binaries/
endif

.PHONY: release-darwin
release-darwin: | $(DIST_DIRS)
	CGO_CFLAGS="$(CGO_CFLAGS)" $(GO) run $(XGO_PACKAGE) -go $(XGO_VERSION) -dest $(DIST)/binaries -tags 'netgo osusergo $(TAGS)' -ldflags '$(LDFLAGS)' -targets '$(DARWIN_ARCHS)' -out $(EXECUTABLE)-$(VERSION) .
ifeq ($(CI),true)
	cp -r /build/* $(DIST)/binaries/
endif

.PHONY: release-copy
release-copy: | $(DIST_DIRS)
	cd $(DIST); for file in `find . -type f -name "*"`; do cp $${file} ./release/; done;

.PHONY: release-check
release-check: | $(DIST_DIRS)
	cd $(DIST)/release/; for file in `find . -type f -name "*"`; do echo "checksumming $${file}" && $(SHASUM) `echo $${file} | sed 's/^..//'` > $${file}.sha256; done;

.PHONY: release-compress
release-compress: | $(DIST_DIRS)
	cd $(DIST)/release/; for file in `find . -type f -name "*"`; do echo "compressing $${file}" && $(GO) run $(GXZ_PAGAGE) -k -9 $${file}; done;

.PHONY: docker
docker:
	if ! docker buildx version >/dev/null 2>&1; then \
		ARG_DISABLE_CONTENT_TRUST=--disable-content-trust=false; \
	fi; \
	docker build $${ARG_DISABLE_CONTENT_TRUST} -t $(DOCKER_REF) .
	docker build $${ARG_DISABLE_CONTENT_TRUST} -t $(DOCKER_ROOTLESS_REF) -f Dockerfile.rootless .

clean:
	$(GO) clean -x -i ./...
	rm -rf coverage.txt $(EXECUTABLE) $(DIST)

version:
	@echo $(VERSION)
