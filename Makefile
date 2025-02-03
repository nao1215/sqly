.PHONY: build test clean vet fmt chkfmt

APP         = sqly
VERSION     = $(shell git describe --tags --abbrev=0)
GO          = go
GO_BUILD    = $(GO) build
GO_FORMAT   = $(GO) fmt
GOFMT       = gofmt
GO_LIST     = $(GO) list
GO_TEST     = $(GO) test -v
GO_TOOL     = $(GO) tool
GO_VET      = $(GO) vet
GO_DEP      = $(GO) mod
GO_INSTALL  = $(GO) install
GOOS        = ""
GOARCH      = ""
GO_PKGROOT  = ./...
GO_PACKAGES = $(shell $(GO_LIST) $(GO_PKGROOT))
GO_LDFLAGS  = -ldflags '-X github.com/nao1215/sqly/config.Version=${VERSION}'

build:  ## Build binary
	env GO111MODULE=on CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) $(GO_BUILD) $(GO_LDFLAGS) -o $(APP) main.go

clean: ## Clean project
	-rm -rf $(APP) cover.*

test: ## Start test
	env GOOS=$(GOOS) $(GO_TEST) -cover $(GO_PKGROOT) -coverpkg=./... -coverprofile=cover.out
	$(GO_TOOL) cover -html=cover.out -o cover.html

bench: ## Start benchmark
	env GOOS=$(GOOS) go test -bench=BenchmarkImport100000Records -benchmem

coverage-tree: test ## Generate coverage tree
	grep -v 'github.com/nao1215/sqly/interactor/mock' cover.out | grep -v 'github.com/nao1215/sqly/infrastructure/mock' > cover.tmp
	go-cover-treemap -statements -percent -coverprofile cover.tmp > doc/img/cover-tree.svg

changelog: ## Generate changelog
	ghch --format markdown > CHANGELOG.md

generate: ## Generate code from templates
	$(GO) generate ./...

tools: ## Install dependency tools 
	$(GO_INSTALL) github.com/Songmu/ghch/cmd/ghch@latest
	$(GO_INSTALL) github.com/nao1215/hottest@latest
	$(GO_INSTALL) github.com/google/wire/cmd/wire@latest
	$(GO_INSTALL) github.com/charmbracelet/vhs@latest
	$(GO_INSTALL) github.com/nikolaydubina/go-cover-treemap@latest
	$(GO_INSTALL) github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	$(GO_INSTALL) go.uber.org/mock/mockgen@latest
	$(GO_INSTALL) github.com/fe3dback/go-arch-lint@latest

lint: ## Lint code
	golangci-lint run --config .golangci.yml
	go-arch-lint check

.DEFAULT_GOAL := help
help:  
	@grep -E '^[0-9a-zA-Z_-]+[[:blank:]]*:.*?## .*$$' $(MAKEFILE_LIST) | sort \
	| awk 'BEGIN {FS = ":.*?## "}; {printf "\033[1;32m%-15s\033[0m %s\n", $$1, $$2}'
