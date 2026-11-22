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
	python3 -m unittest discover -s portable -p '*_test.py'
	@GOBUILD='$(GOBUILD)' sh build_test.sh
	@sh session_start_hook_test.sh

install: $(BINARIES)
	install -d $(PREFIX)/bin
	install -m 755 $(BINARIES) $(PREFIX)/bin/

# The portable version: a POSIX sh front end for the prompt's subcommands
# and Python 3 for the rest, for machines that can't run the Go binaries.
# It installs as `vcs`, in place of the Go one.
PORTABLE_COMMIT = $(shell git rev-parse HEAD 2>/dev/null || echo unknown)
PORTABLE_DATE = $(shell git log -1 --format=%cI 2>/dev/null || echo unknown)

install-portable:
	install -d $(PREFIX)/bin
	install -m 755 portable/vcs $(PREFIX)/bin/vcs
	sed -e 's/@COMMIT@/$(PORTABLE_COMMIT)/' -e 's/@DATE@/$(PORTABLE_DATE)/' \
		portable/vcs-py >$(PREFIX)/bin/vcs-py.tmp
	chmod 755 $(PREFIX)/bin/vcs-py.tmp
	mv $(PREFIX)/bin/vcs-py.tmp $(PREFIX)/bin/vcs-py
	# The Go backends a previous `make install` left would never be called,
	# and on a machine that blocks them they'd only confuse.
	rm -f $(addprefix $(PREFIX)/bin/,$(filter-out vcs,$(BINARIES)))

clean:
	rm -f $(BINARIES)

.PHONY: all test install install-portable clean
