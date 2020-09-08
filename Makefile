#----------------------------------------------------------------------------------
# Base
#----------------------------------------------------------------------------------

ROOTDIR := $(shell pwd)
OUTPUT_DIR ?= $(ROOTDIR)/_output

# If you just put your username, then that refers to your account at hub.docker.com
ifeq ($(IMAGE_REPO),) # Set quay.io/solo-io as default if IMAGE_REPO is unset
	IMAGE_REPO := quay.io/solo-io
endif

# Kind of a hack to make sure _output exists
z := $(shell mkdir -p $(OUTPUT_DIR))

SOURCES := $(shell find . -name "*.go" | grep -v test.go)
RELEASE := "true"
ifeq ($(TAGGED_VERSION),)
	TAGGED_VERSION := $(shell git describe --tags --dirty)
	RELEASE := "false"
endif
VERSION ?= $(shell echo $(TAGGED_VERSION) | cut -c 2-)

# WASM version has '-wasm' added after major.minor.patch but before label. Eg 1.2.3-wasm or 1.2.3-wasm-rc1
WASM_VERSION ?= $(shell echo $(VERSION) | sed 's/\([0-9]\{1,\}\.[0-9]\{1,\}\.[0-9]\{1,\}\)/\1-wasm/g')

# For non-versioned releases like local or dev builds, just prepend 'wasm-', eg wasm-dev
ifeq ($(VERSION), $(WASM_VERSION))
	WASM_VERSION = wasm-$(VERSION)
endif

ENVOY_GLOO_IMAGE ?= quay.io/solo-io/envoy-gloo:1.16.0-rc2
ENVOY_GLOO_WASM_IMAGE ?= quay.io/solo-io/envoy-gloo:1.15.0-wasm-rc1

# The full SHA of the currently checked out commit
CHECKED_OUT_SHA := $(shell git rev-parse HEAD)
# Returns the name of the default branch in the remote `origin` repository, e.g. `master`
DEFAULT_BRANCH_NAME := $(shell git symbolic-ref refs/remotes/origin/HEAD | sed 's@^refs/remotes/origin/@@')
# Print the branches that contain the current commit and keep only the one that
# EXACTLY matches the name of the default branch (avoid matching e.g. `master-2`).
# If we get back a result, it mean we are on the default branch.
EMPTY_IF_NOT_DEFAULT := $(shell git branch --contains $(CHECKED_OUT_SHA) | grep -ow $(DEFAULT_BRANCH_NAME))

ON_DEFAULT_BRANCH := false
ifneq ($(EMPTY_IF_NOT_DEFAULT),)
    ON_DEFAULT_BRANCH = true
endif

ASSETS_ONLY_RELEASE := true
ifeq ($(ON_DEFAULT_BRANCH), true)
    ASSETS_ONLY_RELEASE = false
endif

print-git-info:
	@echo CHECKED_OUT_SHA: $(CHECKED_OUT_SHA)
	@echo DEFAULT_BRANCH_NAME: $(DEFAULT_BRANCH_NAME)
	@echo EMPTY_IF_NOT_DEFAULT: $(EMPTY_IF_NOT_DEFAULT)
	@echo ON_DEFAULT_BRANCH: $(ON_DEFAULT_BRANCH)
	@echo ASSETS_ONLY_RELEASE: $(ASSETS_ONLY_RELEASE)

LDFLAGS := "-X github.com/solo-io/gloo/pkg/version.Version=$(VERSION)"
GCFLAGS := all="-N -l"

# Define Architecture. Default: amd64
# If GOARCH is unset, docker-build will fail
ifeq ($(GOARCH),)
	GOARCH := amd64
endif

GO_BUILD_FLAGS := GO111MODULE=on CGO_ENABLED=0 GOARCH=$(GOARCH)

# Passed by cloudbuild
GCLOUD_PROJECT_ID := $(GCLOUD_PROJECT_ID)
BUILD_ID := $(BUILD_ID)

TEST_ASSET_DIR := $(ROOTDIR)/_test

#----------------------------------------------------------------------------------
# Macros
#----------------------------------------------------------------------------------

# This macro takes a relative path as its only argument and returns all the files
# in the tree rooted at that directory that match the given criteria.
get_sources = $(shell find $(1) -name "*.go" | grep -v test | grep -v generated.go | grep -v mock_)

#----------------------------------------------------------------------------------
# Repo setup
#----------------------------------------------------------------------------------

# https://www.viget.com/articles/two-ways-to-share-git-hooks-with-your-team/
.PHONY: init
init:
	git config core.hooksPath .githooks

.PHONY: fmt-changed
fmt-changed:
	git diff --name-only | grep '.*.go$$' | xargs -- goimports -w

# must be a seperate target so that make waits for it to complete before moving on
.PHONY: mod-download
mod-download:
	go mod download

DEPSGOBIN=$(shell pwd)/_output/.bin

# https://github.com/go-modules-by-example/index/blob/master/010_tools/README.md
.PHONY: install-go-tools
install-go-tools: mod-download
	mkdir -p $(DEPSGOBIN)
	chmod +x $(shell go list -f '{{ .Dir }}' -m k8s.io/code-generator)/generate-groups.sh
	GOBIN=$(DEPSGOBIN) go install github.com/solo-io/protoc-gen-ext
	GOBIN=$(DEPSGOBIN) go install golang.org/x/tools/cmd/goimports
	GOBIN=$(DEPSGOBIN) go install github.com/gogo/protobuf/protoc-gen-gogo
	GOBIN=$(DEPSGOBIN) go install github.com/cratonica/2goarray
	GOBIN=$(DEPSGOBIN) go install github.com/golang/mock/gomock
	GOBIN=$(DEPSGOBIN) go install github.com/golang/mock/mockgen
	GOBIN=$(DEPSGOBIN) go install github.com/gogo/protobuf/gogoproto
	GOBIN=$(DEPSGOBIN) go install github.com/onsi/ginkgo/ginkgo

# command to run regression tests with guaranteed access to $(DEPSGOBIN)/ginkgo
# requires the environment variable KUBE2E_TESTS to be set to the test type you wish to run
.PHONY: run-ci-regression-tests
run-ci-regression-tests: install-go-tools
	$(DEPSGOBIN)/ginkgo -r -failFast -trace -progress -race -compilers=4 -failOnPending -noColor ./test/kube2e/...

.PHONY: check-format
check-format:
	NOT_FORMATTED=$$(gofmt -l ./projects/ ./pkg/ ./test/) && if [ -n "$$NOT_FORMATTED" ]; then echo These files are not formatted: $$NOT_FORMATTED; exit 1; fi

check-spelling:
	./ci/spell.sh check
#----------------------------------------------------------------------------------
# Clean
#----------------------------------------------------------------------------------

# Important to clean before pushing new releases. Dockerfiles and binaries may not update properly
.PHONY: clean
clean:
	rm -rf _output
	rm -rf _test
	rm -rf docs/site*
	rm -rf docs/themes
	rm -rf docs/resources
	git clean -f -X install

# This is required to run if making changes to proto files then run `make generated-code -B`
clean-generated-code:
	rm -rf projects/gloo/pkg/plugins/grpc/*.descriptor.go

#----------------------------------------------------------------------------------
# Generated Code and Docs
#----------------------------------------------------------------------------------

.PHONY: generated-code
generated-code: $(OUTPUT_DIR)/.generated-code verify-enterprise-protos update-licenses generate-helm-files init

# Note: currently we generate CLI docs, but don't push them to the consolidated docs repo (gloo-docs). Instead, the
# Glooctl enterprise docs are pushed from the private repo.
# TODO(EItanya): make mockgen work for gloo
SUBDIRS:=$(shell ls -d -- */ | grep -v vendor)
$(OUTPUT_DIR)/.generated-code:
	go mod tidy
	find * -type f -name '*.sk.md' -exec rm {} \;
	rm -rf vendor_any
	PATH=$(DEPSGOBIN):$$PATH GO111MODULE=on go generate ./...
	PATH=$(DEPSGOBIN):$$PATH rm docs/content/reference/cli/glooctl*; GO111MODULE=on go run projects/gloo/cli/cmd/docs/main.go
	PATH=$(DEPSGOBIN):$$PATH gofmt -w $(SUBDIRS)
	PATH=$(DEPSGOBIN):$$PATH goimports -w $(SUBDIRS)
	mkdir -p $(OUTPUT_DIR)
	touch $@

# Make sure that the enterprise API *.pb.go files that are generated but not used in this repo are valid.
.PHONY: verify-enterprise-protos
verify-enterprise-protos:
	@echo Verifying validity of generated enterprise files...
	$(GO_BUILD_FLAGS) GOOS=linux go build projects/gloo/pkg/api/v1/enterprise/verify.go

#----------------------------------------------------------------------------------
# Generate mocks
#----------------------------------------------------------------------------------

# The values in this array are used in a foreach loop to dynamically generate the
# commands in the generate-client-mocks target.
# For each value, the ":" character will be replaced with " " using the subst function,
# thus turning the string into a 3-element array. The n-th element of the array will
# then be selected via the word function
MOCK_RESOURCE_INFO := \
	gloo:artifact:ArtifactClient \
	gloo:endpoint:EndpointClient \
	gloo:proxy:ProxyClient \
	gloo:secret:SecretClient \
	gloo:settings:SettingsClient \
	gloo:upstream:UpstreamClient \
	gateway:gateway:GatewayClient \
	gateway:virtual_service:VirtualServiceClient\
	gateway:route_table:RouteTableClient\

# Use gomock (https://github.com/golang/mock) to generate mocks for our resource clients.
.PHONY: generate-client-mocks
generate-client-mocks:
	@$(foreach INFO, $(MOCK_RESOURCE_INFO), \
		echo Generating mock for $(word 3,$(subst :, , $(INFO)))...; \
		GOBIN=$(DEPSGOBIN) mockgen -destination=projects/$(word 1,$(subst :, , $(INFO)))/pkg/mocks/mock_$(word 2,$(subst :, , $(INFO)))_client.go \
     		-package=mocks \
     		github.com/solo-io/gloo/projects/$(word 1,$(subst :, , $(INFO)))/pkg/api/v1 \
     		$(word 3,$(subst :, , $(INFO))) \
     	;)

#----------------------------------------------------------------------------------
# glooctl
#----------------------------------------------------------------------------------

CLI_DIR=projects/gloo/cli

$(OUTPUT_DIR)/glooctl: $(SOURCES)
	GO111MODULE=on go build -ldflags=$(LDFLAGS) -gcflags=$(GCFLAGS) -o $@ $(CLI_DIR)/cmd/main.go

$(OUTPUT_DIR)/glooctl-linux-$(GOARCH): $(SOURCES)
	$(GO_BUILD_FLAGS) GOOS=linux go build -ldflags=$(LDFLAGS) -gcflags=$(GCFLAGS) -o $@ $(CLI_DIR)/cmd/main.go

$(OUTPUT_DIR)/glooctl-darwin-$(GOARCH): $(SOURCES)
	$(GO_BUILD_FLAGS) GOOS=darwin go build -ldflags=$(LDFLAGS) -gcflags=$(GCFLAGS) -o $@ $(CLI_DIR)/cmd/main.go

$(OUTPUT_DIR)/glooctl-windows-$(GOARCH).exe: $(SOURCES)
	$(GO_BUILD_FLAGS) GOOS=windows go build -ldflags=$(LDFLAGS) -gcflags=$(GCFLAGS) -o $@ $(CLI_DIR)/cmd/main.go


.PHONY: glooctl
glooctl: $(OUTPUT_DIR)/glooctl
.PHONY: glooctl-linux-$(GOARCH)
glooctl-linux-$(GOARCH): $(OUTPUT_DIR)/glooctl-linux-$(GOARCH)
.PHONY: glooctl-darwin-$(GOARCH)
glooctl-darwin-$(GOARCH): $(OUTPUT_DIR)/glooctl-darwin-$(GOARCH)
.PHONY: glooctl-windows-$(GOARCH)
glooctl-windows-$(GOARCH): $(OUTPUT_DIR)/glooctl-windows-$(GOARCH).exe

.PHONY: build-cli
build-cli: glooctl-linux-$(GOARCH) glooctl-darwin-$(GOARCH) glooctl-windows-$(GOARCH)

#----------------------------------------------------------------------------------
# Gateway
#----------------------------------------------------------------------------------

GATEWAY_DIR=projects/gateway
GATEWAY_SOURCES=$(call get_sources,$(GATEWAY_DIR))
GATEWAY_OUTPUT_DIR=$(OUTPUT_DIR)/$(GATEWAY_DIR)

$(GATEWAY_OUTPUT_DIR)/gateway-linux-$(GOARCH): $(GATEWAY_SOURCES)
	$(GO_BUILD_FLAGS) GOOS=linux go build -ldflags=$(LDFLAGS) -gcflags=$(GCFLAGS) -o $@ $(GATEWAY_DIR)/cmd/main.go

.PHONY: gateway
gateway: $(GATEWAY_OUTPUT_DIR)/gateway-linux-$(GOARCH)

$(GATEWAY_OUTPUT_DIR)/Dockerfile.gateway: $(GATEWAY_DIR)/cmd/Dockerfile
	cp $< $@

gateway-docker: $(GATEWAY_OUTPUT_DIR)/gateway-linux-$(GOARCH) $(GATEWAY_OUTPUT_DIR)/Dockerfile.gateway
	docker build $(GATEWAY_OUTPUT_DIR) -f $(GATEWAY_OUTPUT_DIR)/Dockerfile.gateway \
		--build-arg GOARCH=$(GOARCH) \
		-t $(IMAGE_REPO)/gateway:$(VERSION)

#----------------------------------------------------------------------------------
# Ingress
#----------------------------------------------------------------------------------

INGRESS_DIR=projects/ingress
INGRESS_SOURCES=$(call get_sources,$(INGRESS_DIR))
INGRESS_OUTPUT_DIR=$(OUTPUT_DIR)/$(INGRESS_DIR)

$(INGRESS_OUTPUT_DIR)/ingress-linux-$(GOARCH): $(INGRESS_SOURCES)
	$(GO_BUILD_FLAGS) GOOS=linux go build -ldflags=$(LDFLAGS) -gcflags=$(GCFLAGS) -o $@ $(INGRESS_DIR)/cmd/main.go

.PHONY: ingress
ingress: $(INGRESS_OUTPUT_DIR)/ingress-linux-$(GOARCH)

$(INGRESS_OUTPUT_DIR)/Dockerfile.ingress: $(INGRESS_DIR)/cmd/Dockerfile
	cp $< $@

ingress-docker: $(INGRESS_OUTPUT_DIR)/ingress-linux-$(GOARCH) $(INGRESS_OUTPUT_DIR)/Dockerfile.ingress
	docker build $(INGRESS_OUTPUT_DIR) -f $(INGRESS_OUTPUT_DIR)/Dockerfile.ingress \
		--build-arg GOARCH=$(GOARCH) \
		-t $(IMAGE_REPO)/ingress:$(VERSION)

#----------------------------------------------------------------------------------
# Access Logger
#----------------------------------------------------------------------------------

ACCESS_LOG_DIR=projects/accesslogger
ACCESS_LOG_SOURCES=$(call get_sources,$(ACCESS_LOG_DIR))
ACCESS_LOG_OUTPUT_DIR=$(OUTPUT_DIR)/$(ACCESS_LOG_DIR)

$(ACCESS_LOG_OUTPUT_DIR)/access-logger-linux-$(GOARCH): $(ACCESS_LOG_SOURCES)
	$(GO_BUILD_FLAGS) GOOS=linux go build -ldflags=$(LDFLAGS) -gcflags=$(GCFLAGS) -o $@ $(ACCESS_LOG_DIR)/cmd/main.go

.PHONY: access-logger
access-logger: $(ACCESS_LOG_OUTPUT_DIR)/access-logger-linux-$(GOARCH)

$(ACCESS_LOG_OUTPUT_DIR)/Dockerfile.access-logger: $(ACCESS_LOG_DIR)/cmd/Dockerfile
	cp $< $@

access-logger-docker: $(ACCESS_LOG_OUTPUT_DIR)/access-logger-linux-$(GOARCH) $(ACCESS_LOG_OUTPUT_DIR)/Dockerfile.access-logger
	docker build $(ACCESS_LOG_OUTPUT_DIR) -f $(ACCESS_LOG_OUTPUT_DIR)/Dockerfile.access-logger \
		--build-arg GOARCH=$(GOARCH) \
		-t $(IMAGE_REPO)/access-logger:$(VERSION)

#----------------------------------------------------------------------------------
# Discovery
#----------------------------------------------------------------------------------

DISCOVERY_DIR=projects/discovery
DISCOVERY_SOURCES=$(call get_sources,$(DISCOVERY_DIR))
DISCOVERY_OUTPUT_DIR=$(OUTPUT_DIR)/$(DISCOVERY_DIR)

$(DISCOVERY_OUTPUT_DIR)/discovery-linux-$(GOARCH): $(DISCOVERY_SOURCES)
	$(GO_BUILD_FLAGS) GOOS=linux go build -ldflags=$(LDFLAGS) -gcflags=$(GCFLAGS) -o $@ $(DISCOVERY_DIR)/cmd/main.go

.PHONY: discovery
discovery: $(DISCOVERY_OUTPUT_DIR)/discovery-linux-$(GOARCH)

$(DISCOVERY_OUTPUT_DIR)/Dockerfile.discovery: $(DISCOVERY_DIR)/cmd/Dockerfile
	cp $< $@

discovery-docker: $(DISCOVERY_OUTPUT_DIR)/discovery-linux-$(GOARCH) $(DISCOVERY_OUTPUT_DIR)/Dockerfile.discovery
	docker build $(DISCOVERY_OUTPUT_DIR) -f $(DISCOVERY_OUTPUT_DIR)/Dockerfile.discovery \
		--build-arg GOARCH=$(GOARCH) \
		-t $(IMAGE_REPO)/discovery:$(VERSION)

#----------------------------------------------------------------------------------
# Gloo
#----------------------------------------------------------------------------------

GLOO_DIR=projects/gloo
GLOO_SOURCES=$(call get_sources,$(GLOO_DIR))
GLOO_OUTPUT_DIR=$(OUTPUT_DIR)/$(GLOO_DIR)

$(GLOO_OUTPUT_DIR)/gloo-linux-$(GOARCH): $(GLOO_SOURCES)
	$(GO_BUILD_FLAGS) GOOS=linux go build -ldflags=$(LDFLAGS) -gcflags=$(GCFLAGS) -o $@ $(GLOO_DIR)/cmd/main.go

.PHONY: gloo
gloo: $(GLOO_OUTPUT_DIR)/gloo-linux-$(GOARCH)

$(GLOO_OUTPUT_DIR)/Dockerfile.gloo: $(GLOO_DIR)/cmd/Dockerfile
	cp hack/utils/oss_compliance/third_party_licenses.txt $(GLOO_OUTPUT_DIR)/third_party_licenses.txt
	cp $< $@

gloo-docker: $(GLOO_OUTPUT_DIR)/gloo-linux-$(GOARCH) $(GLOO_OUTPUT_DIR)/Dockerfile.gloo
	docker build $(GLOO_OUTPUT_DIR) -f $(GLOO_OUTPUT_DIR)/Dockerfile.gloo \
		--build-arg GOARCH=$(GOARCH) \
		--build-arg ENVOY_IMAGE=$(ENVOY_GLOO_IMAGE) \
		-t $(IMAGE_REPO)/gloo:$(VERSION)

#----------------------------------------------------------------------------------
# SDS Server - gRPC server for serving Secret Discovery Service config for Gloo MTLS
#----------------------------------------------------------------------------------

SDS_DIR=projects/sds
SDS_SOURCES=$(call get_sources,$(SDS_DIR))
SDS_OUTPUT_DIR=$(OUTPUT_DIR)/$(SDS_DIR)

$(SDS_OUTPUT_DIR)/sds-linux-$(GOARCH): $(SDS_SOURCES)
	$(GO_BUILD_FLAGS) GOOS=linux go build -ldflags=$(LDFLAGS) -gcflags=$(GCFLAGS) -o $@ $(SDS_DIR)/cmd/main.go

.PHONY: sds
sds: $(SDS_OUTPUT_DIR)/sds-linux-$(GOARCH)

$(SDS_OUTPUT_DIR)/Dockerfile.sds: $(SDS_DIR)/cmd/Dockerfile
	cp $< $@

.PHONY: sds-docker
sds-docker: $(SDS_OUTPUT_DIR)/sds-linux-$(GOARCH) $(SDS_OUTPUT_DIR)/Dockerfile.sds
	docker build $(SDS_OUTPUT_DIR) -f $(SDS_OUTPUT_DIR)/Dockerfile.sds \
		--build-arg GOARCH=$(GOARCH) \
		-t $(IMAGE_REPO)/sds:$(VERSION)

#----------------------------------------------------------------------------------
# Envoy init (BASE/SIDECAR)
#----------------------------------------------------------------------------------

ENVOYINIT_DIR=projects/envoyinit/cmd
ENVOYINIT_SOURCES=$(call get_sources,$(ENVOYINIT_DIR))
ENVOYINIT_OUTPUT_DIR=$(OUTPUT_DIR)/$(ENVOYINIT_DIR)

$(ENVOYINIT_OUTPUT_DIR)/envoyinit-linux-$(GOARCH): $(ENVOYINIT_SOURCES)
	$(GO_BUILD_FLAGS) GOOS=linux go build -ldflags=$(LDFLAGS) -gcflags=$(GCFLAGS) -o $@ $(ENVOYINIT_DIR)/main.go

.PHONY: envoyinit
envoyinit: $(ENVOYINIT_OUTPUT_DIR)/envoyinit-linux-$(GOARCH)

$(ENVOYINIT_OUTPUT_DIR)/Dockerfile.envoyinit: $(ENVOYINIT_DIR)/Dockerfile.envoyinit
	cp $< $@

$(ENVOYINIT_OUTPUT_DIR)/docker-entrypoint.sh: $(ENVOYINIT_DIR)/docker-entrypoint.sh
	cp $< $@

.PHONY: gloo-envoy-wrapper-docker
gloo-envoy-wrapper-docker: $(ENVOYINIT_OUTPUT_DIR)/envoyinit-linux-$(GOARCH) $(ENVOYINIT_OUTPUT_DIR)/Dockerfile.envoyinit $(ENVOYINIT_OUTPUT_DIR)/docker-entrypoint.sh
	docker build $(ENVOYINIT_OUTPUT_DIR) -f $(ENVOYINIT_OUTPUT_DIR)/Dockerfile.envoyinit \
		--build-arg GOARCH=$(GOARCH) \
		--build-arg ENVOY_IMAGE=$(ENVOY_GLOO_IMAGE) \
		-t $(IMAGE_REPO)/gloo-envoy-wrapper:$(VERSION)

#----------------------------------------------------------------------------------
# Envoy init (WASM)
#----------------------------------------------------------------------------------

ENVOY_WASM_DIR=projects/envoyinit/cmd
ENVOY_WASM_SOURCES=$(call get_sources,$(ENVOY_WASM_DIR))
ENVOY_WASM_OUTPUT_DIR=$(OUTPUT_DIR)/$(ENVOY_WASM_DIR)

$(ENVOY_WASM_OUTPUT_DIR)/envoywasm-linux-$(GOARCH): $(ENVOY_WASM_SOURCES)
	$(GO_BUILD_FLAGS) GOOS=linux go build -ldflags=$(LDFLAGS) -gcflags=$(GCFLAGS) -o $@ $(ENVOY_WASM_DIR)/main.go

.PHONY: envoywasm
envoywasm: $(ENVOY_WASM_OUTPUT_DIR)/envoywasm-linux-$(GOARCH)

$(ENVOY_WASM_OUTPUT_DIR)/Dockerfile.envoywasm: $(ENVOY_WASM_DIR)/Dockerfile.envoywasm
	cp $< $@

.PHONY: gloo-envoy-wasm-wrapper-docker
gloo-envoy-wasm-wrapper-docker: $(ENVOY_WASM_OUTPUT_DIR)/envoywasm-linux-$(GOARCH) $(ENVOY_WASM_OUTPUT_DIR)/Dockerfile.envoywasm
	docker build $(ENVOY_WASM_OUTPUT_DIR) -f $(ENVOY_WASM_OUTPUT_DIR)/Dockerfile.envoywasm \
		--build-arg GOARCH=$(GOARCH) \
		--build-arg ENVOY_IMAGE=$(ENVOY_GLOO_WASM_IMAGE) \
		-t $(IMAGE_REPO)/gloo-envoy-wrapper:$(WASM_VERSION)

#----------------------------------------------------------------------------------
# Certgen - Job for creating TLS Secrets in Kubernetes
#----------------------------------------------------------------------------------

CERTGEN_DIR=jobs/certgen/cmd
CERTGEN_SOURCES=$(call get_sources,$(CERTGEN_DIR))
CERTGEN_OUTPUT_DIR=$(OUTPUT_DIR)/$(CERTGEN_DIR)

$(CERTGEN_OUTPUT_DIR)/certgen-linux-$(GOARCH): $(CERTGEN_SOURCES)
	$(GO_BUILD_FLAGS) GOOS=linux go build -ldflags=$(LDFLAGS) -gcflags=$(GCFLAGS) -o $@ $(CERTGEN_DIR)/main.go

.PHONY: certgen
certgen: $(CERTGEN_OUTPUT_DIR)/certgen-linux-$(GOARCH)

$(CERTGEN_OUTPUT_DIR)/Dockerfile.certgen: $(CERTGEN_DIR)/Dockerfile
	cp $< $@

.PHONY: certgen-docker
certgen-docker: $(CERTGEN_OUTPUT_DIR)/certgen-linux-$(GOARCH) $(CERTGEN_OUTPUT_DIR)/Dockerfile.certgen
	docker build $(CERTGEN_OUTPUT_DIR) -f $(CERTGEN_OUTPUT_DIR)/Dockerfile.certgen \
		--build-arg GOARCH=$(GOARCH) \
		-t $(IMAGE_REPO)/certgen:$(VERSION)

#----------------------------------------------------------------------------------
# Build All
#----------------------------------------------------------------------------------
.PHONY: build
build: gloo glooctl gateway discovery envoyinit certgen ingress

#----------------------------------------------------------------------------------
# Deployment Manifests / Helm
#----------------------------------------------------------------------------------

HELM_SYNC_DIR := $(OUTPUT_DIR)/helm
HELM_DIR := install/helm/gloo
HELM_BUCKET := gs://solo-public-helm

# Creates Chart.yaml and values.yaml. See install/helm/README.md for more info.
.PHONY: generate-helm-files
generate-helm-files: $(OUTPUT_DIR)/.helm-prepared

HELM_PREPARED_INPUT := $(HELM_DIR)/generate.go $(wildcard $(HELM_DIR)/generate/*.go)
$(OUTPUT_DIR)/.helm-prepared: $(HELM_PREPARED_INPUT)
	mkdir -p $(HELM_SYNC_DIR)/charts
	go run $(HELM_DIR)/generate.go --version $(VERSION) --generate-helm-docs
	touch $@

package-chart: generate-helm-files
	mkdir -p $(HELM_SYNC_DIR)/charts
	helm package --destination $(HELM_SYNC_DIR)/charts $(HELM_DIR)
	helm repo index $(HELM_SYNC_DIR)

push-chart-to-registry: generate-helm-files
	mkdir -p $(HELM_REPOSITORY_CACHE)
	cp $(DOCKER_CONFIG)/config.json $(HELM_REPOSITORY_CACHE)/config.json
	HELM_EXPERIMENTAL_OCI=1 helm chart save $(HELM_DIR) gcr.io/solo-public/gloo-helm:$(VERSION)
	HELM_EXPERIMENTAL_OCI=1 helm chart push gcr.io/solo-public/gloo-helm:$(VERSION)

.PHONY: fetch-package-and-save-helm
fetch-package-and-save-helm: generate-helm-files
ifeq ($(RELEASE),"true")
	until $$(GENERATION=$$(gsutil ls -a $(HELM_BUCKET)/index.yaml | tail -1 | cut -f2 -d '#') && \
					gsutil cp -v $(HELM_BUCKET)/index.yaml $(HELM_SYNC_DIR)/index.yaml && \
					helm package --destination $(HELM_SYNC_DIR)/charts $(HELM_DIR) >> /dev/null && \
					helm repo index $(HELM_SYNC_DIR) --merge $(HELM_SYNC_DIR)/index.yaml && \
					gsutil -m rsync $(HELM_SYNC_DIR)/charts $(HELM_BUCKET)/charts && \
					gsutil -h x-goog-if-generation-match:"$$GENERATION" cp $(HELM_SYNC_DIR)/index.yaml $(HELM_BUCKET)/index.yaml); do \
		echo "Failed to upload new helm index (updated helm index since last download?). Trying again"; \
		sleep 2; \
	done
endif

#----------------------------------------------------------------------------------
# Build the Gloo Manifests that are published as release assets
#----------------------------------------------------------------------------------

.PHONY: render-manifests
render-manifests: install/gloo-gateway.yaml install/gloo-ingress.yaml install/gloo-knative.yaml

INSTALL_NAMESPACE ?= gloo-system

MANIFEST_OUTPUT = > /dev/null
ifneq ($(BUILD_ID),)
MANIFEST_OUTPUT =
endif

define HELM_VALUES
namespace:
  create: true
crds:
  create: true
endef

# Export as a shell variable, make variables do not play well with multiple lines
export HELM_VALUES
$(OUTPUT_DIR)/release-manifest-values.yaml:
	@echo "$$HELM_VALUES" > $@

install/gloo-gateway.yaml: $(OUTPUT_DIR)/glooctl-linux-$(GOARCH) $(OUTPUT_DIR)/release-manifest-values.yaml package-chart
ifeq ($(RELEASE),"true")
	$(OUTPUT_DIR)/glooctl-linux-$(GOARCH) install gateway -n $(INSTALL_NAMESPACE) -f $(HELM_SYNC_DIR)/charts/gloo-$(VERSION).tgz \
		--values $(OUTPUT_DIR)/release-manifest-values.yaml --dry-run | tee $@ $(OUTPUT_YAML) $(MANIFEST_OUTPUT)
endif

install/gloo-knative.yaml: $(OUTPUT_DIR)/glooctl-linux-$(GOARCH) $(OUTPUT_DIR)/release-manifest-values.yaml package-chart
ifeq ($(RELEASE),"true")
	$(OUTPUT_DIR)/glooctl-linux-$(GOARCH) install knative -n $(INSTALL_NAMESPACE) -f $(HELM_SYNC_DIR)/charts/gloo-$(VERSION).tgz \
		--values $(OUTPUT_DIR)/release-manifest-values.yaml --dry-run | tee $@ $(OUTPUT_YAML) $(MANIFEST_OUTPUT)
endif

install/gloo-ingress.yaml: $(OUTPUT_DIR)/glooctl-linux-$(GOARCH) $(OUTPUT_DIR)/release-manifest-values.yaml package-chart
ifeq ($(RELEASE),"true")
	$(OUTPUT_DIR)/glooctl-linux-$(GOARCH) install ingress -n $(INSTALL_NAMESPACE) -f $(HELM_SYNC_DIR)/charts/gloo-$(VERSION).tgz \
		--values $(OUTPUT_DIR)/release-manifest-values.yaml --dry-run | tee $@ $(OUTPUT_YAML) $(MANIFEST_OUTPUT)
endif

#----------------------------------------------------------------------------------
# Release
#----------------------------------------------------------------------------------

$(OUTPUT_DIR)/gloo-enterprise-version:
	GO111MODULE=on go run hack/find_latest_enterprise_version.go

# The code does the proper checking for a TAGGED_VERSION
.PHONY: upload-github-release-assets
upload-github-release-assets: print-git-info build-cli render-manifests
	GO111MODULE=on go run ci/upload_github_release_assets.go $(ASSETS_ONLY_RELEASE)

.PHONY: publish-docs
publish-docs: generate-helm-files
	cd docs && make docker-push-docs \
		VERSION=$(VERSION) \
		TAGGED_VERSION=$(TAGGED_VERSION) \
		GCLOUD_PROJECT_ID=$(GCLOUD_PROJECT_ID) \
		RELEASE=$(RELEASE) \
		ON_DEFAULT_BRANCH=$(ON_DEFAULT_BRANCH)


#----------------------------------------------------------------------------------
# Docker
#----------------------------------------------------------------------------------
#
#---------
#--------- Push
#---------

DOCKER_IMAGES :=
ifeq ($(RELEASE),"true")
	DOCKER_IMAGES := docker
endif

.PHONY: docker docker-push
docker: discovery-docker gateway-docker gloo-docker \
 		gloo-envoy-wrapper-docker gloo-envoy-wasm-wrapper-docker \
		certgen-docker sds-docker ingress-docker access-logger-docker

# Depends on DOCKER_IMAGES, which is set to docker if RELEASE is "true", otherwise empty (making this a no-op).
# This prevents executing the dependent targets if RELEASE is not true, while still enabling `make docker`
# to be used for local testing.
# docker-push is intended to be run by CI
docker-push: $(DOCKER_IMAGES)
	docker push $(IMAGE_REPO)/gateway:$(VERSION) && \
	docker push $(IMAGE_REPO)/ingress:$(VERSION) && \
	docker push $(IMAGE_REPO)/discovery:$(VERSION) && \
	docker push $(IMAGE_REPO)/gloo:$(VERSION) && \
	docker push $(IMAGE_REPO)/gloo-envoy-wrapper:$(VERSION) && \
	docker push $(IMAGE_REPO)/gloo-envoy-wrapper:$(WASM_VERSION) && \
	docker push $(IMAGE_REPO)/certgen:$(VERSION) && \
	docker push $(IMAGE_REPO)/sds:$(VERSION) && \
	docker push $(IMAGE_REPO)/access-logger:$(VERSION)

CLUSTER_NAME ?= kind

push-kind-images: docker
	kind load docker-image $(IMAGE_REPO)/gateway:$(VERSION) --name $(CLUSTER_NAME)
	kind load docker-image $(IMAGE_REPO)/ingress:$(VERSION) --name $(CLUSTER_NAME)
	kind load docker-image $(IMAGE_REPO)/discovery:$(VERSION) --name $(CLUSTER_NAME)
	kind load docker-image $(IMAGE_REPO)/gloo:$(VERSION) --name $(CLUSTER_NAME)
	kind load docker-image $(IMAGE_REPO)/gloo-envoy-wrapper:$(VERSION) --name $(CLUSTER_NAME)
	kind load docker-image $(IMAGE_REPO)/gloo-envoy-wrapper:$(WASM_VERSION) --name $(CLUSTER_NAME)
	kind load docker-image $(IMAGE_REPO)/certgen:$(VERSION) --name $(CLUSTER_NAME)
	kind load docker-image $(IMAGE_REPO)/access-logger:$(VERSION) --name $(CLUSTER_NAME)
	kind load docker-image $(IMAGE_REPO)/sds:$(VERSION) --name $(CLUSTER_NAME)


#----------------------------------------------------------------------------------
# Build assets for Kube2e tests
#----------------------------------------------------------------------------------
#
# The following targets are used to generate the assets on which the kube2e tests rely upon. The following actions are performed:
#
#   1. Generate Gloo value files
#   2. Package the Gloo Helm chart to the _test directory (also generate an index file)
#
# The Kube2e tests will use the generated Gloo Chart to install Gloo to the GKE test cluster.

.PHONY: build-test-assets
build-test-assets: build-test-chart $(OUTPUT_DIR)/glooctl-linux-$(GOARCH) \
 	$(OUTPUT_DIR)/glooctl-darwin-$(GOARCH)

.PHONY: build-kind-assets
build-kind-assets: push-kind-images build-kind-chart $(OUTPUT_DIR)/glooctl-linux-$(GOARCH) \
 	$(OUTPUT_DIR)/glooctl-darwin-$(GOARCH)

.PHONY: build-test-chart
build-test-chart:
	mkdir -p $(TEST_ASSET_DIR)
	GO111MODULE=on go run $(HELM_DIR)/generate.go --version $(VERSION)
	helm package --destination $(TEST_ASSET_DIR) $(HELM_DIR)
	helm repo index $(TEST_ASSET_DIR)

.PHONY: build-kind-chart
build-kind-chart:
	rm -rf $(TEST_ASSET_DIR)
	mkdir -p $(TEST_ASSET_DIR)
	GO111MODULE=on go run $(HELM_DIR)/generate.go --version $(VERSION)
	helm package --destination $(TEST_ASSET_DIR) $(HELM_DIR)
	helm repo index $(TEST_ASSET_DIR)

#----------------------------------------------------------------------------------
# Third Party License Management
#----------------------------------------------------------------------------------
.PHONY: update-licenses
update-licenses:
# TODO(helm3): fix after we completely drop toml parsing in favor of go modules
#	cd hack/utils/oss_compliance && GO111MODULE=on go run main.go


#----------------------------------------------------------------------------------
# Printing makefile variables utility
#----------------------------------------------------------------------------------

# use `make print-MAKEFILE_VAR` to print the value of MAKEFILE_VAR

print-%  : ; @echo $($*)
