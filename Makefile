BINARIES = vcs vcs-git vcs-hg vcs-jj

# Default PREFIX: ~/.local for non-root, /usr/local for root.
ifeq ($(shell id -u),0)
PREFIX ?= /usr/local
else
PREFIX ?= $(HOME)/.local
endif

SOURCES = $(shell find . -name '*.go' -not -name '*_test.go')

# Build metadata injected into the version package via -ldflags.
# Overridable on the command line (e.g. `make VERSION=v1.2.3`).
VERSION    ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT     ?= $(shell git rev-parse HEAD 2>/dev/null || echo unknown)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

VERSION_PKG = github.com/mikelward/vcs/version
LDFLAGS = -X $(VERSION_PKG).Version=$(VERSION) \
          -X $(VERSION_PKG).Commit=$(COMMIT) \
          -X $(VERSION_PKG).BuildDate=$(BUILD_DATE)
GO_BUILD = go build -ldflags '$(LDFLAGS)'

all: $(BINARIES)

# Real file targets so Make skips the build when sources are unchanged.
vcs: $(SOURCES)
	$(GO_BUILD) -o $@ ./cmd/vcs

vcs-git: $(SOURCES)
	$(GO_BUILD) -o $@ ./cmd/vcs-git

vcs-hg: $(SOURCES)
	$(GO_BUILD) -o $@ ./cmd/vcs-hg

vcs-jj: $(SOURCES)
	$(GO_BUILD) -o $@ ./cmd/vcs-jj

test:
	go test ./...

install: $(BINARIES)
	install -d $(PREFIX)/bin
	install -m 755 $(BINARIES) $(PREFIX)/bin/

clean:
	rm -f $(BINARIES)

.PHONY: all test install clean
