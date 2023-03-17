#
# Copyright 2023 IBM Corporation
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

# ====================================================================================
# Project Setup
PROJECT_NAME := provider-kubernetes
PROJECT_REPO := github.com/crossplane-contrib/$(PROJECT_NAME)

# IBM Crossplane supported platforms
PLATFORMS ?= linux_amd64 linux_ppc64le linux_s390x

# -include will silently skip missing files, which allows us
# to load those files with a target in the Makefile. If only
# "include" was used, the make command would fail and refuse
# to run a target until the include commands succeeded.
-include build/makelib/common.mk

# ====================================================================================
# Setup Output

-include build/makelib/output.mk

# ====================================================================================
# Setup Go

# Set a sane default so that the nprocs calculation below is less noisy on the initial
# loading of this file
NPROCS ?= 1

# Setup golang-ci version

GOLANGCILINT_VERSION=1.51.2

# each of our test suites starts a kube-apiserver and running many test suites in
# parallel can lead to high CPU utilization. by default we reduce the parallelism
# to half the number of CPU cores.
GO_TEST_PARALLEL := $(shell echo $$(( $(NPROCS) / 2 )))

GO_STATIC_PACKAGES = $(GO_PROJECT)/cmd/provider
GO_SUBDIRS += cmd internal apis
GO111MODULE = on
-include build/makelib/golang.mk

# ====================================================================================
# Setup tools
MANIFEST_TOOL_VERSION = v1.0.3
BUILDX_VERSION = v0.6.1
KIND_VERSION = v0.11.1
USE_HELM3 = true
-include build/makelib/k8s_tools.mk

# ====================================================================================
# Setup Images
export RELEASE_VERSION=$(shell cat RELEASE_VERSION)
export VERSION=$(RELEASE_VERSION)
export GIT_VERSION=$(shell git describe --exact-match 2> /dev/null || \
                 	   git describe --match=$(git rev-parse --short=8 HEAD) --always --dirty --abbrev=8)

DOCKER_REGISTRY = docker-na-public.artifactory.swg-devops.com/hyc-cloud-private-scratch-docker-local/ibmcom
IMAGES = provider-kubernetes provider-kubernetes-controller
-include build/makelib/image.mk

# ====================================================================================
# Setup Local Dev
-include build/makelib/local.mk

# ====================================================================================
# Include operator targets
-include common/Makefile.operator.mk

# ====================================================================================
# Targets

# run `make help` to see the targets and options

# We want submodules to be set up the first time `make` is run.
# We manage the build/ folder and its Makefiles as a submodule.
# The first time `make` is run, the includes of build/*.mk files will
# all fail, and this target will be run. The next time, the default as defined
# by the includes will be run instead.
fallthrough: submodules
	@echo Initial setup complete. Running make again . . .
	@make

# Generate a coverage report for cobertura applying exclusions on
# - generated file
cobertura:
	@cat $(GO_TEST_OUTPUT)/coverage.txt | \
		grep -v zz_generated.deepcopy | \
		$(GOCOVER_COBERTURA) > $(GO_TEST_OUTPUT)/cobertura-coverage.xml

crds.clean:
	@$(INFO) cleaning generated CRDs
	@find package/crds -name *.yaml -exec sed -i.sed -e '1,2d' {} \; || $(FAIL)
	@find package/crds -name *.yaml.sed -delete || $(FAIL)
	@$(OK) cleaned generated CRDs

generate.done: crds.clean

# integration tests
e2e.run: test-integration

local-dev: local.up local.deploy.crossplane

# Run integration tests.
test-integration: $(KIND) $(KUBECTL) $(HELM3)
	@$(INFO) running integration tests using kind $(KIND_VERSION)
	@$(ROOT_DIR)/cluster/integration/integration_tests.sh || $(FAIL)
	@$(OK) integration tests passed

# Update the submodules, such as the common build scripts.
submodules:
	@git submodule sync
	@git submodule update --init --recursive

# This is for running out-of-cluster locally, and is for convenience. Running
# this make target will print out the command which was used. For more control,
# try running the binary directly with different arguments.
run: $(KUBECTL) generate
	@$(INFO) Running Crossplane locally out-of-cluster . . .
	@$(KUBECTL) apply -f package/crds/ -R
	go run cmd/provider/main.go -d

manifests:
	@$(INFO) Deprecated. Run make generate instead.

.PHONY: cobertura submodules fallthrough test-integration run manifests crds.clean

# ====================================================================================
# IBM Customization
-include ibm/Makefile.common.mk

