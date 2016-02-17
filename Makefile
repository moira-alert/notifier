VERSION := $(shell sh -c 'git describe --always --tags')

default: test build

build:
	go build -ldflags "-X main.Version=$(VERSION)" github.com/moira-alert/notifier/notifier

test: prepare
	ginkgo -r --randomizeAllSpecs --randomizeSuites --failOnPending --trace --race --progress tests

.PHONY: test

prepare:
	go get github.com/sparrc/gdm
	gdm restore
	go get github.com/onsi/ginkgo/ginkgo
