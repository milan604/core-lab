.PHONY: build build-cmd version

build:
	@$(MAKE) -f build/Makefile build

build-cmd:
	@$(MAKE) -f build/Makefile build-cmd

version:
	@$(MAKE) -f build/Makefile version
