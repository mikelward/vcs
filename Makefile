BINARIES = vcs vcs-git vcs-hg vcs-jj vcs-p4

# Default PREFIX: ~/.local for non-root, /usr/local for root.
ifeq ($(shell id -u),0)
PREFIX ?= /usr/local
else
PREFIX ?= $(HOME)/.local
endif

# -trimpath keeps the build directory (usually under $HOME) out of the
# binaries and makes builds reproducible: one commit and Go version give
# the same bytes on any machine.
GOBUILD = go build -trimpath

SOURCES = $(shell find . -name '*.go' -not -name '*_test.go')

all: $(BINARIES)

# Real file targets so Make skips the build when sources are unchanged.
# They depend on the Makefile too, so a change to GOBUILD rebuilds them.
vcs: $(SOURCES) Makefile
	$(GOBUILD) -o $@ ./cmd/vcs

vcs-git: $(SOURCES) Makefile
	$(GOBUILD) -o $@ ./cmd/vcs-git

vcs-hg: $(SOURCES) Makefile
	$(GOBUILD) -o $@ ./cmd/vcs-hg

vcs-jj: $(SOURCES) Makefile
	$(GOBUILD) -o $@ ./cmd/vcs-jj

vcs-p4: $(SOURCES) Makefile
	$(GOBUILD) -o $@ ./cmd/vcs-p4

test:
	go test ./...
	@GOBUILD='$(GOBUILD)' sh build_test.sh
	@sh session_start_hook_test.sh

install: $(BINARIES)
	install -d $(PREFIX)/bin
	install -m 755 $(BINARIES) $(PREFIX)/bin/

clean:
	rm -f $(BINARIES)

.PHONY: all test install clean
