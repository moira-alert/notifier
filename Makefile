VERSION := $(shell sh -c 'git describe --always --tags')
VENDOR := "SKB Kontur"
URL := "https://github.com/moira-alert"
LICENSE := "GPLv3"

default: test build

build:
	go build -ldflags "-X main.Version=$(VERSION)" -o build/moira-notifier github.com/moira-alert/notifier/notifier

test: prepare
	ginkgo -r --randomizeAllSpecs --randomizeSuites --failOnPending --trace --race --progress tests

.PHONY: test

prepare:
	go get github.com/sparrc/gdm
	gdm restore
	go get github.com/onsi/ginkgo/ginkgo

clean:
	rm -rf build

rpm: clean build
	mkdir -p build/root/usr/local/bin
	mkdir -p build/root/usr/lib/systemd/system
	mkdir -p build/root/etc/logrotate.d/

	mv build/moira-notifier build/root/usr/local/bin/
	cp pkg/rpm/moira-notifier.service build/root/usr/lib/systemd/system/moira-notifier.service
	cp pkg/logrotate build/root/etc/logrotate.d/moira-notifier

	fpm -t rpm \
		-s "dir" \
		--description "Moira Notifier" \
		-C build/root \
		--vendor $(VENDOR) \
		--url $(URL) \
		--license $(LICENSE) \
		--name "moira-notifier" \
		--version "$(VERSION)" \
		--config-files "/usr/lib/systemd/system/moira-notifier.service" \
		--after-install "./pkg/rpm/postinst" \
		--depends logrotate \
		-p build
