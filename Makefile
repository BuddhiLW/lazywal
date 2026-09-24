PREFIX ?= /usr
VERSION := v$(shell tr -d '[:space:]' < VERSION)
LDFLAGS := -X github.com/BuddhiLW/lazywal/loop.Version=$(VERSION)

build:
	@go build -ldflags "$(LDFLAGS)" -o lazywal ./cmd/lazywal
	@go build -ldflags "$(LDFLAGS)" -o lazywal-mcp ./cmd/lazywal-mcp

install: build
	@mkdir -p $(DESTDIR)$(PREFIX)/bin
	@cp -p lazywal $(DESTDIR)$(PREFIX)/bin/lazywal
	@cp -p lazywal-mcp $(DESTDIR)$(PREFIX)/bin/lazywal-mcp
	@chmod 755 $(DESTDIR)$(PREFIX)/bin/lazywal
	@chmod 755 $(DESTDIR)$(PREFIX)/bin/lazywal-mcp
	@bash $(PWD)/auto-completion.bash

uninstall:
	@rm -rf $(DESTDIR)$(PREFIX)/bin/lazywal
	@rm -rf $(DESTDIR)$(PREFIX)/bin/lazywal-mcp

#for debug purposes
link:
	@ln -s $(realpath lazywal) $(DESTDIR)$(PREFIX)/bin/lazywal
	@ln -s $(realpath lazywal-mcp) $(DESTDIR)$(PREFIX)/bin/lazywal-mcp

.PHONY: build install uninstall link
