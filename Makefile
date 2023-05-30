PROJECT_NAME := amplify-tool
PKG_LIST := $(shell go list ./...)

.PHONY: clean

_all: clean build ## Build everything

all: clean build

build: ## Build the binary for linux
	go build -o ./bin/$(PROJECT_NAME)

clean: ## Clean out dir
	rm -rf ./bin

help: ## Display this help screen
	@grep	-h	-E	'^[a-zA-Z_-]+:.*?## .*$$'	$(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

