VERSION := $(shell git describe --always --tags --abbrev=0 | tail -c+2)
RELEASE := $(shell git describe --always --tags | awk -F- '{ if ($$2) dot="."} END { printf "1%s%s%s%s\n",dot,$$2,dot,$$3}')
VENDOR := "SKB Kontur"
URL := "https://github.com/moira-alert"
LICENSE := "GPLv3"

export GOPATH := $(CURDIR)/_vendor

default: clean test build

build:
	go build -ldflags "-X main.Version=$(VERSION)-$(RELEASE)" -o build/moira-notifier github.com/moira-alert/notifier/notifier

test: prepare
	$(GOPATH)/bin/ginkgo -r --randomizeAllSpecs --randomizeSuites -cover -coverpkg=../ --failOnPending --trace --race --progress tests

.PHONY: test

prepare:
	mkdir -p _vendor/src/github.com/moira-alert
	ln -s $(CURDIR) _vendor/src/github.com/moira-alert/notifier || true
	go get github.com/AlexAkulov/gdm
	$(GOPATH)/bin/gdm restore
	go get github.com/onsi/ginkgo/ginkgo

clean:
	rm -rf build

tar:
	mkdir -p build/root/usr/local/bin
	mkdir -p build/root/usr/lib/systemd/system
	mkdir -p build/root/etc/logrotate.d/

	mv build/moira-notifier build/root/usr/local/bin/
	cp pkg/moira-notifier.service build/root/usr/lib/systemd/system/moira-notifier.service
	cp pkg/logrotate build/root/etc/logrotate.d/moira-notifier

	tar -czvPf build/moira-notifier-$(VERSION)-$(RELEASE).tar.gz -C build/root .

rpm:
	fpm -t rpm \
		-s "tar" \
		--description "Moira Notifier" \
		--vendor $(VENDOR) \
		--url $(URL) \
		--license $(LICENSE) \
		--name "moira-notifier" \
		--version "$(VERSION)" \
		--iteration "$(RELEASE)" \
		--after-install "./pkg/postinst" \
		--depends logrotate \
		-p build \
		build/moira-notifier-$(VERSION)-$(RELEASE).tar.gz
deb:
	fpm -t deb \
		-s "tar" \
		--description "Moira Notifier" \
		--vendor $(VENDOR) \
		--url $(URL) \
		--license $(LICENSE) \
		--name "moira-notifier" \
		--version "$(VERSION)" \
		--iteration "$(RELEASE)" \
		--after-install "./pkg/postinst" \
		--depends logrotate \
		-p build \
		build/moira-notifier-$(VERSION)-$(RELEASE).tar.gz

packages: clean build tar rpm deb
