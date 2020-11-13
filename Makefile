DOCKER_REPO             ?= "quay.io/samsahai/samsahai"
DOCKER_REGISTRY         ?= "quay.io"
DOCKER_USER             ?=
DOCKER_PASSWORD         ?=

GITHUB_API_URL          ?= https://api.github.com
GITHUB_TOKEN            ?=
GITHUB_REPO             ?= agoda-com/samsahai
GO_VERSION              ?= 1.15.4
GOLANGCI_LINT_VERSION   ?= 1.32.2

GO                      ?= go

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell $(GO) env GOBIN))
GOBIN=$(shell $(GO) env GOPATH)/bin
else
GOBIN=$(shell $(GO) env GOBIN)
endif
KUBEBUILDER_VERSION     ?= 2.2.0
KUBEBULIDER_FILENAME    = kubebuilder_$(KUBEBUILDER_VERSION)_$(OS)_$(ARCH)
KUBEBUILDER_PATH        ?= /usr/local/kubebuilder/
GORELEASER_VERSION      ?= 0.124.1
K3S_DOCKER_IMAGE        ?= rancher/k3s:v1.18.10-k3s2
KUBECONFIG              = /tmp/s2h/k3s-kubeconfig
K3S_DOCKER_NAME         ?= s2h-k3s-server
K3S_PORT                ?= 7443
K8S_VERSION             ?= 1.18.10
KUSTOMIZE_VERSION       ?= 3.8.6
HELM_VERSION            ?= 3.3.4
POD_NAMESPACE           ?= default

GO111MODULE             := on
SUDO                    ?=
INSTALL_DIR             ?= $(PWD)/bin/
OS                      = $$(echo `uname`|tr '[:upper:]' '[:lower:]')
OS2                     = $$(if [ "$$(uname|tr '[:upper:]' '[:lower:]')" = "linux" ]; then echo linux; elif [ "$$(uname|tr '[:upper:]' '[:lower:]')" = "darwin" ]; then echo osx; fi)
ARCH                    = $$(if [ "$$(uname -m)" = "x86" ]; then echo 386; elif [ "$$(uname -m)" = "x86_64" ]; then echo amd64; fi)
ARCHx86                 = $$(if [ "$$(uname -m)" = "x86" ]; then echo x86_32; elif [ "$$(uname -m)" = "x86_64" ]; then echo x86_64; fi)
DEBUG                   ?=
ARCHIVE_EXT             ?= .tar.gz
TMP_DIR                 ?= /tmp/samsahai

MV                      = $(SUDO)mv
RM                      = $(SUDO)rm
MKDIR                   = $(SUDO)mkdir
CHMOD                   = $(SUDO)chmod
CHOWN                   = $(SUDO)chown
CURL                    = $(SUDO)curl
TAR                     = $(SUDO)tar
DOCKER                  ?= docker
K3S_EXEC                = $(DOCKER) exec -i $(K3S_DOCKER_NAME)
KUBECTL                 = $(INSTALL_DIR)kubectl
KUSTOMIZE               = $(INSTALL_DIR)kustomize
HELM                    = $(INSTALL_DIR)helm
PROTOC                  = $(INSTALL_DIR)protoc
GORELEASER              = $(INSTALL_DIR)goreleaser
KUBEBUILDER             = $(KUBEBUILDER_PATH)bin/kubebuilder
GOLANGCI_LINT           = $(INSTALL_DIR)golangci-lint
PROTOC                  = $(INSTALL_DIR)protoc
SWAG                    = $(INSTALL_DIR)swag

PASS_PROXY              ?=
ifdef PASS_PROXY
K3S_DOCKER_ARGS         ?= -e http_proxy=$(http_proxy) -e https_proxy=$(https_proxy) -e no_proxy=$(no_proxy)
else
K3S_DOCKER_ARGS         ?=
endif

.PHONY: init
init: tidy install-dep

.PHONY: install-dep
install-dep: .install-kubectl .install-kustomize .install-golangci-lint .install-kubebuilder .install-helm \
			.install-protoc .install-swag .install-gotools
	@echo 'done!'

.PHONY: format
format:
	GO111MODULE=off gofmt -w .

.PHONY: lint
lint: format tidy
	GO111MODULE=on $(GOLANGCI_LINT) run --timeout 5m

.PHONY: tidy
tidy:
	GO111MODULE=on $(GO) mod tidy

.PHONY: golangci-lint-check-version
golangci-lint-check-version:
	@if $(GOLANGCI_LINT) --version | grep "$(GOLANGCI_LINT_VERSION)" > /dev/null; then \
		echo; \
	else \
		echo "golangci-lint version mismatch"; \
		exit 1; \
	fi

#
# Testing
#

.PHONY: unit-test e2e-test prepare-env-e2e \
	prepare-env-e2e-k3d e2e-test-k3d \
	overall-coverage coverage-html

ifndef DEBUG
.SILENT: unit-test \
	prepare-env-e2e e2e-test \
	overall-coverage coverage-html generate-rpc
endif

unit-test: format lint swag
	./scripts/unit-test.sh

e2e-test:
	export KUBECONFIG=$(KUBECONFIG); \
	export POD_NAMESPACE=$(POD_NAMESPACE); \
	./scripts/e2e-test.sh;

e2e-test-k3d: e2e-test

k3s-get-kubeconfig:
	$(MKDIR) -p $(shell dirname $(KUBECONFIG))
	$(K3S_EXEC) cat /output/kubeconfig > $(KUBECONFIG)
	@echo export KUBECONFIG=$(KUBECONFIG)

prepare-env-e2e-k3d: prepare-env-e2e

prepare-kube-1-13: K3S_DOCKER_IMAGE = rancher/k3s:v0.8.1
prepare-kube-1-13: prepare-env-e2e

prepare-env-e2e:
	echo start k3s

	if $(DOCKER) ps | grep $(K3S_DOCKER_NAME); then \
		echo $(K3S_DOCKER_NAME) is running..; \
	else \
		$(DOCKER) rm -f $(K3S_DOCKER_NAME) || echo; \
		$(DOCKER) run --name $(K3S_DOCKER_NAME) -p $(K3S_PORT):$(K3S_PORT) \
			-e K3S_KUBECONFIG_OUTPUT=/output/kubeconfig \
			-e K3S_CLUSTER_SECRET=123456 \
			$(K3S_DOCKER_ARGS) \
			--privileged -d \
			$(K3S_DOCKER_IMAGE) server --https-listen-port $(K3S_PORT); \
	fi

	until $(K3S_EXEC) kubectl get node | grep -i "ready" >/dev/null; do sleep 2; done

	echo waiting for cluster to be ready...

	set -e; \
	export KUBECONFIG=$(KUBECONFIG); \
	$(MKDIR) -p $$(dirname $(KUBECONFIG)); \
	$(K3S_EXEC) cat /output/kubeconfig > $(KUBECONFIG); \
	$(KUBECTL) version; \
	\
	until $(KUBECTL) -n kube-system get pods -l k8s-app=kube-dns 2>&1 | grep -iv "no resources found" >/dev/null; do sleep 1; done; \
	$(KUBECTL) -n kube-system wait pods -l k8s-app=kube-dns --for=condition=Ready --timeout=5m; \
	until $(KUBECTL) -n kube-system get pods -l app=svclb-traefik 2>&1 | grep -iv "no resources found" >/dev/null; do sleep 1; done; \
	$(KUBECTL) -n kube-system wait pods -l app=svclb-traefik --for=condition=Ready --timeout=5m; \
	until $(KUBECTL) -n kube-system get pods -l app=traefik 2>&1 | grep -iv "no resources found" >/dev/null; do sleep 1; done; \
	$(KUBECTL) -n kube-system wait pods -l app=traefik --for=condition=Ready --timeout=5m; \
	$(KUBECTL) create ns samsahai-system || echo 'namespace "samsahai-system" already exist'; \
	\
	\
	echo install helm and helm-operator; \
	\
	$(HELM) repo add stable https://kubernetes-charts.storage.googleapis.com; \
	$(HELM) repo add incubator https://kubernetes-charts-incubator.storage.googleapis.com; \
	\
	echo create sa,clusterrole,clusterrolebinding for samsahai; \
	$(HELM) template -n "samsahai-system" --set "fullnameOverride=samsahai" $(PWD)/config/chart/samsahai -s templates/sa.yaml | $(KUBECTL) apply -n samsahai-system -f - ; \
	$(HELM) template -n "samsahai-system" --set "fullnameOverride=samsahai" $(PWD)/config/chart/samsahai -s templates/clusterrole-rbac.yaml | $(KUBECTL) apply -n samsahai-system -f - ; \
	\
	echo $(PWD); \
	$(KUBECTL) apply -f $(PWD)/config/crds;

	echo done!

coverage-html:
	$(GO) tool cover -html=coverage.txt -o coverage.html

overall-coverage:
	 @echo "Overall Coverage: $$($(GO) tool cover -func=coverage.txt|tail -1|awk '{print $$3}')"


#
# Release
#

GIT_TREE_STATE	= $(shell if [ -z "`git status --porcelain`" ]; then echo "clean" ; else echo "dirty"; fi)
GIT_COMMIT_MSG	?= $(shell git log -n 1 --pretty=format:'%s')
SKIP_PUBLISH	?=

.PHONY: release auto-release

ifndef DEBUG
.SILENT: .release-precheck .docker-login .bump-version .git-tag .git-push .goreleaser .docker-logout
endif

release: .install-kubectl-linux .install-goreleaser .release-precheck \
		.docker-login .bump-version .git-tag .git-push .goreleaser .docker-logout

auto-release:
	@if echo "$(GIT_COMMIT_MSG)" | grep -i '\[major]'; then \
		echo Major release; \
		$(MAKE) release RELEASE_FLAG=-M; \
	elif echo "$(GIT_COMMIT_MSG)" | grep -i '\[minor]'; then \
		echo Minor release; \
		$(MAKE) release RELEASE_FLAG=-m; \
	elif echo "$(GIT_COMMIT_MSG)" | grep -i '\[patch]'; then \
		echo Patch release; \
		$(MAKE) release RELEASE_FLAG=-p; \
	fi;

.release-precheck:
ifndef DRYRUN
	if [ "$(GIT_TREE_STATE)" != "clean" ]; then \
		echo 'git tree state is $(GIT_TREE_STATE)'; \
		git status; \
		echo 'warning: automatic checkout to remove changes'; \
		git checkout .; \
	fi;
endif

.bump-version:
	# get latest tag from github or 'v0.1.0'
	export CURL_ARGS=""; \
	if [ ! -z "$(GITHUB_TOKEN)" ]; then \
		export CURL_ARGS="-H 'Authorization: Bearer $(GITHUB_TOKEN)'"; \
	fi; \
	export LATEST_VERSION=$$(eval $(CURL) --silent -X GET $(GITHUB_API_URL)/repos/$(GITHUB_REPO)/releases/latest $$CURL_ARGS | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/'); \
	if [ -z $$LATEST_VERSION ]; then \
		echo Latest version not found, using v0.1.0; \
		LATEST_VERSION="v0.1.0"; \
	else \
		echo Latest version: $$LATEST_VERSION; \
	fi; \
	./scripts/semver.sh $(RELEASE_FLAG) $$LATEST_VERSION > .version; \
	echo Next version: $$(cat .version);

.goreleaser:
	export GORELEASER_FLAGS=""; \
	if [ ! -z "$(DRYRUN)" ]; then \
		export GORELEASER_FLAGS="--skip-publish --snapshot --debug"; \
	elif [ ! -z "$(SKIP_PUBLISH)" ]; then \
		export GORELEASER_FLAGS="--skip-publish"; \
	fi; \
	export GITHUB_TOKEN=$(GITHUB_TOKEN); \
	export GO_PACKAGE="$(shell $(GO) list ./internal)"; \
	export DOCKER_REPO=$(DOCKER_REPO); \
	export http_proxy="$(http_proxy)"; \
	export https_proxy="$(https_proxy)"; \
	export no_proxy="$(no_proxy)"; \
	eval $(GORELEASER) --rm-dist $$GORELEASER_FLAGS;

.git-tag:
ifndef DRYRUN
	git tag $$(cat .version)
else
	echo git tag $$(cat .version)
endif

.git-push:
ifndef DRYRUN
	git push origin $$(cat .version)
else
	echo git push origin $$(cat .version)
endif

.docker-login:
ifndef NO_DOCKER_LOGIN
	echo $(DOCKER_PASSWORD) | docker login -u $(DOCKER_USER) $(DOCKER_REGISTRY) --password-stdin
endif

.docker-logout:
ifndef NO_DOCKER_LOGIN
	docker logout $(DOCKER_REGISTRY)
endif


#
# Miscellaneous
#

generate-rpc:
	echo $$($(GO) list)
	export PROTO_SRC_PATH=.; \
	export IMPORT_PREFIX="$$($(GO) list)"; \
	$(PROTOC) \
    		--proto_path=$$PROTO_SRC_PATH/:./bin/include/ \
    		--twirp_out=$$PROTO_SRC_PATH \
    		--go_out=$$PROTO_SRC_PATH \
    		$$PROTO_SRC_PATH/pkg/staging/rpc/service.proto; \
	$(PROTOC) \
		--proto_path=$$PROTO_SRC_PATH/:./bin/include/ \
		--twirp_out=$$PROTO_SRC_PATH \
		--go_out=$$PROTO_SRC_PATH \
		$$PROTO_SRC_PATH/pkg/samsahai/rpc/service.proto;

# Generate swag docs
.PHONY: swag
swag:
	$(SWAG) init -g cmd/samsahai/main.go

install-crds: generate manifests
	$(KUBECTL) apply -f ./config/crds
	make lint


APP_NAME 		?=
_APP_CMD 		= $(INSTALL_DIR)$(APP_NAME)
_VERSION_ARGS 	?= version

ifndef DEBUG
.SILENT: .install-kubectl .install-kustomize .install-helm .install-golangci-lint \
			.install-kubebuilder .install-protoc .install-goreleaser .install-kubectl-linux \
			install-go
endif

install-go: export APP_NAME 		= go
install-go: export APP_VERSION 	= $(GO_VERSION)
install-go: export _VERSION_ARGS 	= version
install-go:
	export _FILENAME="go$(GO_VERSION).$(OS)-$(ARCH)"; \
	export _DOWNLOAD_URL="https://storage.googleapis.com/golang/$$_FILENAME.tar.gz"; \
	export _MOVE_CMD="$(RM) -rf /usr/local/$(APP_NAME)/ && $(MKDIR) -p /usr/local/$(APP_NAME)/ && $(MV) $(TMP_DIR)/$(APP_NAME)/* /usr/local/$(APP_NAME)/"; \
	export INSTALL_DIR="/usr/local/$(APP_NAME)/bin/"; \
	$(MAKE) .install-archive;

.install-kubectl: export APP_NAME 		= kubectl
.install-kubectl: export APP_VERSION 	= $(K8S_VERSION)
.install-kubectl: export _VERSION_ARGS 	= version --client --short
.install-kubectl:
	$(MAKE) .install-binary \
		_DOWNLOAD_URL="https://storage.googleapis.com/kubernetes-release/release/v$(K8S_VERSION)/bin/$(OS)/$(ARCH)/kubectl"

.install-kubectl-linux: export APP_NAME			= kubectl-linux
.install-kubectl-linux: export APP_VERSION		= $(K8S_VERSION)
.install-kubectl-linux: export _DOWNLOAD_URL	= https://storage.googleapis.com/kubernetes-release/release/v$(K8S_VERSION)/bin/linux/amd64/kubectl
.install-kubectl-linux:
	$(MKDIR) -p $$(dirname $(_APP_CMD)); \
	$(CURL) -sLo $(APP_NAME) $(_DOWNLOAD_URL); \
	$(CHMOD) +x $(APP_NAME); \
	$(MV) $(APP_NAME) $(_APP_CMD); \
	$(CHOWN) -R $(USER):$(USER) $(_APP_CMD);

.install-kustomize: export APP_NAME 		= kustomize
.install-kustomize: export APP_VERSION 		= $(KUSTOMIZE_VERSION)
.install-kustomize:
	export _FILENAME="kustomize_v$(KUSTOMIZE_VERSION)_$(OS)_$(ARCH)"; \
	export _DOWNLOAD_URL="https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize%2Fv$(KUSTOMIZE_VERSION)/$$_FILENAME.tar.gz"; \
	export _MOVE_CMD="$(MV) $(TMP_DIR)/$(APP_NAME) $(_APP_CMD)"; \
	$(MAKE) .install-archive

.install-golangci-lint: export APP_NAME 		= golangci-lint
.install-golangci-lint: export APP_VERSION 		= $(GOLANGCI_LINT_VERSION)
.install-golangci-lint: export _VERSION_ARGS	= --version
.install-golangci-lint:
	export _FILENAME="golangci-lint-$(GOLANGCI_LINT_VERSION)-$(OS)-$(ARCH)"; \
	export _DOWNLOAD_URL="https://github.com/golangci/golangci-lint/releases/download/v$(GOLANGCI_LINT_VERSION)/$$_FILENAME.tar.gz"; \
	export _MOVE_CMD="$(MV) $(TMP_DIR)/$$_FILENAME/$(APP_NAME) $(_APP_CMD)"; \
	$(MAKE) .install-archive

.install-kubebuilder: export APP_NAME 		= kubebuilder
.install-kubebuilder: export APP_VERSION 	= $(KUBEBUILDER_VERSION)
.install-kubebuilder:
	export _FILENAME="$(KUBEBULIDER_FILENAME)"; \
	export _DOWNLOAD_URL="https://github.com/kubernetes-sigs/kubebuilder/releases/download/v$(KUBEBUILDER_VERSION)/$$_FILENAME.tar.gz"; \
	export _MOVE_CMD="$(MKDIR) -p /usr/local/$(APP_NAME)/bin && $(MV) $(TMP_DIR)/$$_FILENAME/bin/* /usr/local/$(APP_NAME)/bin/"; \
	export INSTALL_DIR="/usr/local/$(APP_NAME)/bin/"; \
	$(MAKE) .install-archive

.install-helm: export APP_NAME 			= helm
.install-helm: export APP_VERSION 		= $(HELM_VERSION)
.install-helm: export _VERSION_ARGS 	= version --client --short
.install-helm:
	export _FILENAME="$(APP_NAME)-v$(HELM_VERSION)-$(OS)-$(ARCH)"; \
	export _DOWNLOAD_URL="https://get.helm.sh/$$_FILENAME.tar.gz"; \
	export _MOVE_CMD="$(CHMOD) +x $(TMP_DIR)/$(OS)-$(ARCH)/$(APP_NAME) && $(MV) $(TMP_DIR)/$(OS)-$(ARCH)/$(APP_NAME) $(_APP_CMD)"; \
	$(MAKE) .install-archive

.install-goreleaser: export APP_NAME 			= goreleaser
.install-goreleaser: export APP_VERSION 		= $(GORELEASER_VERSION)
.install-goreleaser: export _VERSION_ARGS		= --version 2>&1 | tr -s "\n" " "
.install-goreleaser:
	export _FILENAME="$(APP_NAME)_$(OS)_$(ARCHx86)"; \
	export _DOWNLOAD_URL="https://github.com/goreleaser/goreleaser/releases/download/v$(APP_VERSION)/$$_FILENAME.tar.gz"; \
	export _MOVE_CMD="$(CHMOD) +x $(TMP_DIR)/$(APP_NAME) && $(MV) $(TMP_DIR)/$(APP_NAME) $(_APP_CMD)"; \
	$(MAKE) .install-archive;

.install-swag: export APP_NAME 			= swag
.install-swag: export APP_VERSION 		= 1.6.3
.install-swag: export _VERSION_ARGS		= --version
.install-swag:
	export _FILENAME="$(APP_NAME)_$(APP_VERSION)_$(shell uname)_$(ARCHx86)"; \
	export _DOWNLOAD_URL="https://github.com/swaggo/swag/releases/download/v$(APP_VERSION)/$$_FILENAME.tar.gz"; \
	export _MOVE_CMD="$(CHMOD) +x $(TMP_DIR)/$(APP_NAME) && $(MV) $(TMP_DIR)/$(APP_NAME) $(_APP_CMD)"; \
	$(MAKE) .install-archive;

.install-protoc: export APP_NAME 		= protoc
.install-protoc: export APP_VERSION 	= 3.9.1
.install-protoc: export _VERSION_ARGS 	= --version
.install-protoc: export ARCHIVE_EXT 	= .zip
.install-protoc:
	export _FILENAME="$(APP_NAME)-$(APP_VERSION)-$(OS2)-$(ARCHx86)"; \
	export _DOWNLOAD_URL="https://github.com/protocolbuffers/protobuf/releases/download/v$(APP_VERSION)/$$_FILENAME.zip"; \
	export _MOVE_CMD="$(CHMOD) +x $(TMP_DIR)/$$_FILENAME/bin/$(APP_NAME)"; \
	export _MOVE_CMD="$$_MOVE_CMD && $(MV) $(TMP_DIR)/$$_FILENAME/bin/$(APP_NAME) $(_APP_CMD)"; \
	export _MOVE_CMD="$$_MOVE_CMD && $(MV) $(TMP_DIR)/$$_FILENAME/include $(INSTALL_DIR)"; \
	$(MAKE) .install-archive;

ifndef DEBUG
.SILENT: .install-binary .install-archive
endif

.install-binary:
ifdef DEBUG
	@echo "APP_NAME      : $(APP_NAME)"
	@echo "APP_VERSION   : $(APP_VERSION)"
	@echo "OS-ARCH       : $(OS)-$(ARCH)"
	@echo "_APP_CMD      : $(_APP_CMD)"
	@echo "_VERSION_ARGS : $(_VERSION_ARGS)"
	@echo "_DOWNLOAD_URL : $(_DOWNLOAD_URL)"
endif
	echo installing... $(APP_NAME) $(APP_VERSION)
	if $(_APP_CMD) $(_VERSION_ARGS) | grep $(APP_VERSION); then \
		echo $(APP_NAME) $(APP_VERSION) already installed; \
	else \
		$(MKDIR) -p $(TMP_DIR); \
		$(MKDIR) -p $$(dirname $(_APP_CMD)); \
		$(CURL) -sLo $(TMP_DIR)/$(APP_NAME) $(_DOWNLOAD_URL); \
		$(CHMOD) +x $(TMP_DIR)/$(APP_NAME); \
		$(MV) $(TMP_DIR)/$(APP_NAME) $(_APP_CMD); \
		$(_APP_CMD) $(_VERSION_ARGS) | grep $(APP_VERSION) >/dev/null && echo $(APP_NAME) $(APP_VERSION) installed; \
		$(RM) -rf $(TMP_DIR); \
	fi;

.install-archive:
ifdef DEBUG
	@echo "APP_NAME      : $(APP_NAME)"
	@echo "APP_VERSION   : $(APP_VERSION)"
	@echo "OS-ARCH       : $(OS)-$(ARCH)"
	@echo "_APP_CMD      : $(_APP_CMD)"
	@echo "_VERSION_ARGS : $(_VERSION_ARGS)"
	@echo "_FILENAME     : $(_FILENAME)"
	@echo "_DOWNLOAD_URL : $(_DOWNLOAD_URL)"
	@echo "_MOVE_CMD     : $(_MOVE_CMD)"
endif
	echo installing... $(APP_NAME) $(APP_VERSION)
	if [ ! -f $(_APP_CMD) ] || $(_APP_CMD) $(_VERSION_ARGS) | grep -v $(APP_VERSION); then \
		$(MKDIR) -p $(TMP_DIR); \
		$(CURL) -sLo $(TMP_DIR)/$(_FILENAME)$(ARCHIVE_EXT) $(_DOWNLOAD_URL); \
		if [ "$(ARCHIVE_EXT)" = ".tar.gz" ]; then \
			$(TAR) -zxf $(TMP_DIR)/$(_FILENAME)$(ARCHIVE_EXT) -C $(TMP_DIR); \
		elif [ "$(ARCHIVE_EXT)" = ".zip" ]; then \
			unzip $(TMP_DIR)/$(_FILENAME)$(ARCHIVE_EXT) -d $(TMP_DIR)/$(_FILENAME); \
		else \
			echo $(ARCHIVE_EXT) is not support!; \
		fi; \
		\
		$(MKDIR) -p $$(dirname $(_APP_CMD)); \
		$(_MOVE_CMD); \
		\
		\
		echo $(APP_NAME) $(APP_VERSION) installed; \
	else \
		echo $(APP_NAME) $(APP_VERSION) already installed; \
	fi;

.install-gotools:
	@echo installing gotools
	@GO111MODULE=off $(GO) get -u \
		golang.org/x/tools/cmd/goimports \
		github.com/golang/protobuf/protoc-gen-go \
		github.com/twitchtv/twirp/protoc-gen-twirp

# Produce CRDs that work back to Kubernetes 1.18 (no version conversion)
CRD_OPTIONS ?= "crd"

CONTROLLER_GEN=$(GO) run $$GOPATH/pkg/mod/github.com/phantomnat/controller-tools@v0.2.4-1/cmd/controller-gen/main.go

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	$(GO) get sigs.k8s.io/controller-tools
	$(CONTROLLER_GEN) $(CRD_OPTIONS) paths="./..." crd:crdVersions=v1 output:crd:artifacts:config=config/crds output:none
	$(CONTROLLER_GEN) $(CRD_OPTIONS) paths="./..." crd:crdVersions=v1beta1 output:crd:artifacts:config=test/data/crds output:none

# Generate code
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile=./builder/boilerplate.go.txt paths="./..."

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
	@$(GO) get sigs.k8s.io/controller-tools

