PROJECT_NAME := amplify-tool
PKG_LIST := $(shell go list ./...)

.PHONY: clean

_all: clean build ## Build everything

all: clean build

build: ## Build the binary for linux
	go build -ldflags="-X 'github.com/Axway/agent-sdk/pkg/cmd.BuildVersion=1.0.0' \
				-X 'github.com/Axway/agent-sdk/pkg/cmd.SDKBuildVersion=v1.1.53' \
				-X 'github.com/Axway/agent-sdk/pkg/cmd.BuildAgentName=AmplifyTool' \
				-X 'github.com/Axway/agent-sdk/pkg/cmd.BuildAgentDescription=Amplify Tool' \
				-X 'github.com/Axway/agent-sdk/pkg/cmd/service.Name=amplify-tool'" \
				-o ./bin/$(PROJECT_NAME)

	GOOS=windows GOARCH=amd64 go build -ldflags="-X 'github.com/Axway/agent-sdk/pkg/cmd.BuildVersion=1.0.0' \
				-X 'github.com/Axway/agent-sdk/pkg/cmd.SDKBuildVersion=v1.1.53' \
				-X 'github.com/Axway/agent-sdk/pkg/cmd.BuildAgentName=AmplifyTool' \
				-X 'github.com/Axway/agent-sdk/pkg/cmd.BuildAgentDescription=Amplify Tool' \
				-X 'github.com/Axway/agent-sdk/pkg/cmd/service.Name=amplify-tool'" \
				-o ./bin/$(PROJECT_NAME)-windows-amd64
	
	GOOS=windows GOARCH=arm64 go build -ldflags="-X 'github.com/Axway/agent-sdk/pkg/cmd.BuildVersion=1.0.0' \
				-X 'github.com/Axway/agent-sdk/pkg/cmd.SDKBuildVersion=v1.1.53' \
				-X 'github.com/Axway/agent-sdk/pkg/cmd.BuildAgentName=AmplifyTool' \
				-X 'github.com/Axway/agent-sdk/pkg/cmd.BuildAgentDescription=Amplify Tool' \
				-X 'github.com/Axway/agent-sdk/pkg/cmd/service.Name=amplify-tool'" \
				-o ./bin/$(PROJECT_NAME)-windows-arm64
	
	GOOS=linux GOARCH=386 go build -ldflags="-X 'github.com/Axway/agent-sdk/pkg/cmd.BuildVersion=1.0.0' \
				-X 'github.com/Axway/agent-sdk/pkg/cmd.SDKBuildVersion=v1.1.53' \
				-X 'github.com/Axway/agent-sdk/pkg/cmd.BuildAgentName=AmplifyTool' \
				-X 'github.com/Axway/agent-sdk/pkg/cmd.BuildAgentDescription=Amplify Tool' \
				-X 'github.com/Axway/agent-sdk/pkg/cmd/service.Name=amplify-tool'" \
				-o ./bin/$(PROJECT_NAME)-linux-386
	
	GOOS=linux GOARCH=arm64 go build -ldflags="-X 'github.com/Axway/agent-sdk/pkg/cmd.BuildVersion=1.0.0' \
				-X 'github.com/Axway/agent-sdk/pkg/cmd.SDKBuildVersion=v1.1.53' \
				-X 'github.com/Axway/agent-sdk/pkg/cmd.BuildAgentName=AmplifyTool' \
				-X 'github.com/Axway/agent-sdk/pkg/cmd.BuildAgentDescription=Amplify Tool' \
				-X 'github.com/Axway/agent-sdk/pkg/cmd/service.Name=amplify-tool'" \
				-o ./bin/$(PROJECT_NAME)-linux-arm64
	
	GOOS=linux GOARCH=amd64 go build -ldflags="-X 'github.com/Axway/agent-sdk/pkg/cmd.BuildVersion=1.0.0' \
				-X 'github.com/Axway/agent-sdk/pkg/cmd.SDKBuildVersion=v1.1.53' \
				-X 'github.com/Axway/agent-sdk/pkg/cmd.BuildAgentName=AmplifyTool' \
				-X 'github.com/Axway/agent-sdk/pkg/cmd.BuildAgentDescription=Amplify Tool' \
				-X 'github.com/Axway/agent-sdk/pkg/cmd/service.Name=amplify-tool'" \
				-o ./bin/$(PROJECT_NAME)-linux-amd64
clean: ## Clean out dir
	rm -rf ./bin

help: ## Display this help screen
	@grep	-h	-E	'^[a-zA-Z_-]+:.*?## .*$$'	$(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

