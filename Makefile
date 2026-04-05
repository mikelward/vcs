BINARIES = vcs vcs-git vcs-hg vcs-jj

# Default PREFIX: ~/.local for non-root, /usr/local for root.
ifeq ($(shell id -u),0)
PREFIX ?= /usr/local
else
PREFIX ?= $(HOME)/.local
endif

SOURCES = $(shell find . -name '*.go' -not -name '*_test.go')

all: $(BINARIES)

# Real file targets so Make skips the build when sources are unchanged.
vcs: $(SOURCES)
	go build -o $@ ./cmd/vcs

vcs-git: $(SOURCES)
	go build -o $@ ./cmd/vcs-git

vcs-hg: $(SOURCES)
	go build -o $@ ./cmd/vcs-hg

vcs-jj: $(SOURCES)
	go build -o $@ ./cmd/vcs-jj

test:
	go test ./...

install: $(BINARIES)
	install -d $(PREFIX)/bin
	install -m 755 $(BINARIES) $(PREFIX)/bin/

clean:
	rm -f $(BINARIES)

.PHONY: all test install clean
