docker_registry ?= "quay.io"
docker_image_repo ?= "agoda-com/samsahai"
docker_image_tag ?= latest
#docker_user := ""
#docker_password := ""

package_name := github.com/agoda-com/samsahai
app_name := samsahai
output_path := ./out
go_ldflags ?= $(shell govvv -flags -pkg $(shell go list ./internal/samsahai))
golangci_lint_version := 1.17.1
kubebuilder_version := 1.0.8
GO111MODULE ?= on

.PHONY: init
init: tidy install-dep install

.PHONY: install-dep
install-dep:
	GO111MODULE=off go get github.com/ahmetb/govvv
	GO111MODULE=off go get golang.org/x/tools/cmd/goimports
	# install golangci-lint
	@curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | \
		sh -s -- -b $(shell go env GOPATH)/bin v$(golangci_lint_version)

	@echo 'done!'

dep-kubebuilder-osx:
	@version=$(kubebuilder_version) && \
		os=darwin && \
		arch=amd64 && \
		\
		filename=kubebuilder_$${version}_$${os}_$${arch} && \
		echo $$filename && \
		\
		curl -LO "https://github.com/kubernetes-sigs/kubebuilder/releases/download/v$${version}/$${filename}.tar.gz" && \
		\
		tar -zxvf $${filename}.tar.gz && \
		mv $${filename} kubebuilder && \
		sudo rm -rf /usr/local/kubebuilder/ && \
		sudo mv kubebuilder /usr/local/ && \
		rm -f $${filename}

	@echo "Please add '/usr/local/kubebuilder/bin' to PATH"

dep-kubebuilder-linux:
	@version=$(kubebuilder_version) && \
		os=linux && \
		arch=amd64 && \
		\
		filename=kubebuilder_$${version}_$${os}_$${arch} && \
		echo $$filename && \
		\
		curl -LO "https://github.com/kubernetes-sigs/kubebuilder/releases/download/v$${version}/$${filename}.tar.gz" && \
		\
		tar -zxvf $${filename}.tar.gz && \
		mv $${filename} kubebuilder && \
		sudo rm -rf /usr/local/kubebuilder/ && \
		sudo mv kubebuilder /usr/local/ && \
		rm -f $${filename}

.PHONY: format
format:
	hash goimports &> /dev/null || go get golang.org/x/tools/cmd/goimports
	gofmt -w .
	goimports -w .

.PHONY: lint
lint: format
	GO111MODULE=on golangci-lint run

.PHONY: build
build: format
	GO111MODULE=on go build -ldflags="$(go_ldflags)" -o $(output_path)/$(app_name) cmd/main.go

.PHONY: build-plugin-public-registry-checker
build-plugin-public-registry-checker: plugin_name=publicregistry-checker
build-plugin-public-registry-checker: plugin_path=./plugins/publicregistry/
build-plugin-public-registry-checker: build-plugin

.PHONY: install
install: build
	cp $(output_path)/$(app_name) $$GOPATH/bin/$(app_name)

.PHONY: print-flag
print-flag:
	@echo $(go_ldflags)

.PHONY: build-docker
build-docker:
	docker build -t $(docker_registry)/$(docker_image_repo):$(docker_image_tag) \
		--build-arg GO_LDFLAGS="$(go_ldflags)" \
		-f scripts/Dockerfile .

.PHONY: tidy
tidy:
	GO111MODULE=on go mod tidy

.PHONY: golangci-lint-check-version
golangci-lint-check-version:
	@if golangci-lint --version | grep "$(golangci_lint_version)" > /dev/null; then \
		echo; \
	else \
		echo "golangci-lint version mismatch"; \
		exit 1; \
	fi

.PHONY: unit-test
unit-test: format lint
	./scripts/unit-test.sh

.PHONY: e2e-test
e2e-test: install-crds
	./scripts/e2e-test.sh

.PHONY: coverage-html
coverage-html:
	go tool cover -html=coverage.txt

.PHONY: docker-login-b64
docker-login-b64:
	#echo $(docker_password) | base64 --decode | docker login -u $(docker_user) $(docker_registry) --password-stdin

.PHONY: docker-login
docker-login:
	echo $(docker_password) | docker login -u $(docker_user) $(docker_registry) --password-stdin

.PHONY: docker-logout
docker-logout:
	docker logout $(docker_registry)

.PHONY: docker-push
docker-push:
	docker push $(docker_registry)/$(docker_image_repo):$(docker_image_tag)

.PHONY: docker-tag-n-push-latest
docker-tag-n-push-latest:
	@if [ "$(docker_image_tag)" != "latest" ]; then \
		docker tag $(docker_registry)/$(docker_image_repo):$(docker_image_tag) $(docker_registry)/$(docker_image_repo):latest; \
		docker push $(docker_registry)/$(docker_image_repo):latest; \
	fi

# Generate code
generate:
ifndef GOPATH
	$(error GOPATH not defined, please define GOPATH. Run "go help gopath" to learn more about GOPATH)
endif
	#go mod edit -require k8s.io/code-generator@v0.0.0-20190612125529-c522cb6c26aa
	GO111MODULE=off go get k8s.io/code-generator || echo 'ignore error.'
	go generate ./internal/apis/...

manifests:
	go mod edit -require sigs.k8s.io/controller-tools@v0.1.10
	go get sigs.k8s.io/controller-tools
	go run $$GOPATH/pkg/mod/sigs.k8s.io/controller-tools@v0.1.10/cmd/controller-gen/main.go crd --apis-path internal/apis
	go run $$GOPATH/pkg/mod/sigs.k8s.io/controller-tools@v0.1.10/cmd/controller-gen/main.go rbac \
		--name desired-component --input-dir internal/desiredcomponent --output-dir config/rbac/desiredcomponent

install-crds: generate manifests
	kubectl apply -f ./config/crds