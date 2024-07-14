PREFIX ?= /usr

install:
	@mkdir -p $(DESTDIR)$(PREFIX)/bin
	@cp -p lazywal-cli $(DESTDIR)$(PREFIX)/bin/lazywal-cli
	@chmod 755 $(DESTDIR)$(PREFIX)/bin/lazywal-cli

install-go:
	@mkdir -p $(DESTDIR)$(PREFIX)/bin
	@go build -o lazywal $(PWD)/cmd/lazywal/main.go
	@cp -p lazywal $(DESTDIR)$(PREFIX)/bin/lazywal
	@chmod 755 $(DESTDIR)$(PREFIX)/bin/lazywal
	@bash $(PWD)/auto-completion.bash

uninstall:
	@rm -rf $(DESTDIR)$(PREFIX)/bin/lazywal

#for debug purposes
link:
	@ln -s $(realpath lazywal) $(DESTDIR)$(PREFIX)/bin/lazywal
