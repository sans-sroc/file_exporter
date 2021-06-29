BRANCH := $(shell git rev-parse --symbolic-full-name --abbrev-ref HEAD)
SUMMARY := $(shell bash .ci/version)
VERSION := $(shell cat VERSION)
NAME := $(shell basename `pwd`)
MODULE := $(shell cat go.mod | head -n1 | cut -f2 -d' ')
LDFLAGS := "-X $(MODULE)/pkg/common.SUMMARY=$(SUMMARY) -X $(MODULE)/pkg/common.BRANCH=$(BRANCH) -X $(MODULE)/pkg/common.VERSION=$(VERSION)"
OS := $(shell uname -s | awk '{print tolower($$0)}')

.PHONY: build release vendor release-all

vendor:
	go mod vendor

build: vendor
	go build -ldflags $(LDFLAGS) -o $(NAME)

release: vendor
	mkdir -p release
	go build -mod=vendor -ldflags $(LDFLAGS) -o release/$(NAME) .

release-all: vendor
	mkdir -p release
	env GOOS=windows GOARCH=amd64 go build -mod=vendor -ldflags $(LDFLAGS) -o release/$(NAME)_$(SUMMARY)-amd64.exe .
	env GOOS=linux GOARCH=amd64 go build -mod=vendor -ldflags $(LDFLAGS) -o release/$(NAME)_$(SUMMARY)-linux_amd64 .
	env GOOS=darwin GOARCH=amd64 go build -mod=vendor -ldflags $(LDFLAGS) -o release/$(NAME)_$(SUMMARY)-darwin_amd64 .

run-%: vendor
	go run -mod=vendor -ldflags $(LDFLAGS) main.go $*