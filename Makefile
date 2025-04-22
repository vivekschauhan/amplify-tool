PKG_LIST := $(shell go list ./...)
PROJECT_NAME := amplify-tool
AGENT_NAME := AmplifyTool
DESCRIPTION := "Amplify Tool"
SDK_VERSION := "v1.1.155"
BUILD_VERSION := "1.1.0"

.PHONY: clean

_all: clean build ## Build everything

all: clean build

build: ## Build the binary for linux
	go build -ldflags=" \
		-X 'github.com/Axway/agent-sdk/pkg/cmd.BuildVersion=${BUILD_VERSION}' \
		-X 'github.com/Axway/agent-sdk/pkg/cmd.SDKBuildVersion=${SDK_VERSION}' \
		-X 'github.com/Axway/agent-sdk/pkg/cmd.BuildAgentName=${AGENT_NAME}' \
		-X 'github.com/Axway/agent-sdk/pkg/cmd.BuildAgentDescription="${DESCRIPTION}"' \
		-X 'github.com/Axway/agent-sdk/pkg/cmd/service.Name=${PROJECT_NAME}'" \
	-o ./bin/$(PROJECT_NAME)

	GOOS=windows GOARCH=amd64 go build -ldflags=" \
		-X 'github.com/Axway/agent-sdk/pkg/cmd.BuildVersion=${BUILD_VERSION}' \
		-X 'github.com/Axway/agent-sdk/pkg/cmd.SDKBuildVersion=${SDK_VERSION}' \
		-X 'github.com/Axway/agent-sdk/pkg/cmd.BuildAgentName=${AGENT_NAME}' \
		-X 'github.com/Axway/agent-sdk/pkg/cmd.BuildAgentDescription="${DESCRIPTION}"' \
		-X 'github.com/Axway/agent-sdk/pkg/cmd/service.Name=${PROJECT_NAME}'" \
	-o ./bin/$(PROJECT_NAME)-windows-amd64

	GOOS=windows GOARCH=arm64 go build -ldflags=" \
		-X 'github.com/Axway/agent-sdk/pkg/cmd.BuildVersion=${BUILD_VERSION}' \
		-X 'github.com/Axway/agent-sdk/pkg/cmd.SDKBuildVersion=${SDK_VERSION}' \
		-X 'github.com/Axway/agent-sdk/pkg/cmd.BuildAgentName=${AGENT_NAME}' \
		-X 'github.com/Axway/agent-sdk/pkg/cmd.BuildAgentDescription="${DESCRIPTION}"' \
		-X 'github.com/Axway/agent-sdk/pkg/cmd/service.Name=${PROJECT_NAME}'" \
	-o ./bin/$(PROJECT_NAME)-windows-arm64
	
	GOOS=linux GOARCH=386 go build -ldflags=" \
		-X 'github.com/Axway/agent-sdk/pkg/cmd.BuildVersion=${BUILD_VERSION}' \
		-X 'github.com/Axway/agent-sdk/pkg/cmd.SDKBuildVersion=${SDK_VERSION}' \
		-X 'github.com/Axway/agent-sdk/pkg/cmd.BuildAgentName=${AGENT_NAME}' \
		-X 'github.com/Axway/agent-sdk/pkg/cmd.BuildAgentDescription="${DESCRIPTION}"' \
		-X 'github.com/Axway/agent-sdk/pkg/cmd/service.Name=${PROJECT_NAME}'" \
	-o ./bin/$(PROJECT_NAME)-linux-386
	
	GOOS=linux GOARCH=arm64 go build -ldflags=" \
		-X 'github.com/Axway/agent-sdk/pkg/cmd.BuildVersion=${BUILD_VERSION}' \
		-X 'github.com/Axway/agent-sdk/pkg/cmd.SDKBuildVersion=${SDK_VERSION}' \
		-X 'github.com/Axway/agent-sdk/pkg/cmd.BuildAgentName=${AGENT_NAME}' \
		-X 'github.com/Axway/agent-sdk/pkg/cmd.BuildAgentDescription="${DESCRIPTION}"' \
		-X 'github.com/Axway/agent-sdk/pkg/cmd/service.Name=${PROJECT_NAME}'" \
	-o ./bin/$(PROJECT_NAME)-linux-arm64
	
	GOOS=linux GOARCH=amd64 go build -ldflags=" \
		-X 'github.com/Axway/agent-sdk/pkg/cmd.BuildVersion=${BUILD_VERSION}' \
		-X 'github.com/Axway/agent-sdk/pkg/cmd.SDKBuildVersion=${SDK_VERSION}' \
		-X 'github.com/Axway/agent-sdk/pkg/cmd.BuildAgentName=${AGENT_NAME}' \
		-X 'github.com/Axway/agent-sdk/pkg/cmd.BuildAgentDescription="${DESCRIPTION}"' \
		-X 'github.com/Axway/agent-sdk/pkg/cmd/service.Name=${PROJECT_NAME}'" \
	-o ./bin/$(PROJECT_NAME)-linux-amd64
				
clean: ## Clean out dir
	rm -rf ./bin

help: ## Display this help screen
	@grep	-h	-E	'^[a-zA-Z_-]+:.*?## .*$$'	$(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

