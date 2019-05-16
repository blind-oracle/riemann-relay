NAME := riemann-relay
MAINTAINER:= Igor Novgorodov <igor@novg.net>
DESCRIPTION := Service for relaying Riemann events to Riemann/Carbon destinations
LICENSE := MPLv2

GO ?= go
DEP ?= dep
VERSION := $(shell cat VERSION)
OUT := .out
PACKAGE := github.com/blind-oracle/$(NAME)

all: build

build:
	$(DEP) ensure
	$(GO) build $(PACKAGE)

gox-build:
	rm -rf $(OUT)
	mkdir -p $(OUT)
	gox -os="linux" -arch="amd64" -output="$(OUT)/$(NAME)-{{.OS}}-{{.Arch}}" $(PACKAGE)
	mkdir -p $(OUT)/root/etc/$(NAME)
	cp riemann-relay.conf $(OUT)/root/etc/$(NAME)/$(NAME).conf

deb:
	make gox-build
	make build-deb ARCH=amd64

rpm:
	make gox-build
	make build-rpm ARCH=amd64

build-deb:
	fpm -s dir -t deb -n $(NAME) -v $(VERSION) \
		--deb-priority optional \
		--category admin \
		--force \
		--deb-compression bzip2 \
		--url https://$(PACKAGE) \
		--description "$(DESCRIPTION)" \
		-m "$(MAINTAINER)" \
		--license "$(LICENSE)" \
		-a $(ARCH) \
		$(OUT)/$(NAME)-linux-$(ARCH)=/usr/bin/$(NAME) \
		deploy/$(NAME).service=/usr/lib/systemd/system/$(NAME).service \
		$(OUT)/root/=/

build-rpm:
	fpm -s dir -t rpm -n $(NAME) -v $(VERSION) \
		--force \
		--rpm-compression bzip2 \
		--rpm-os linux \
		--url https://$(PACKAGE) \
		--description "$(DESCRIPTION)" \
		-m "$(MAINTAINER)" \
		--license "$(LICENSE)" \
		-a $(ARCH) \
		--config-files /etc/$(NAME)/$(NAME).conf \
		$(OUT)/$(NAME)-linux-$(ARCH)=/usr/bin/$(NAME) \
		deploy/$(NAME).service=/usr/lib/systemd/system/$(NAME).service \
		$(OUT)/root/=/
