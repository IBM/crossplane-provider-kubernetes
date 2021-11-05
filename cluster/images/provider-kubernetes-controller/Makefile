#
# Copyright 2021 IBM Corporation
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
# Setup Project

PLATFORMS := linux_amd64 linux_arm64
include ../../../build/makelib/common.mk

# ====================================================================================
#  Options
IMAGE_NAME = crossplane-provider-kubernetes-controller
IMAGE = $(DOCKER_REGISTRY)/$(IMAGE_NAME):$(RELEASE_VERSION)-$(ARCH)
include ../../../build/makelib/image.mk

# ===================================================================
# Setup Docker buildx

BUILDX = $(shell docker buildx version 2>/dev/null | grep buildx)
BUILDX_VERSION = v0.6.1
BUILDX_PLUGIN = ./bin/docker-buildx
BUILDX_ARGS = --push --platform linux/$(ARCH)

buildx:
ifeq (,$(BUILDX))
	@{ \
	set -e ;\
	mkdir -p bin ;\
	echo "Downloading docker-buildx ...";\
	curl -LO https://github.com/docker/buildx/releases/download/$(BUILDX_VERSION)/buildx-$(BUILDX_VERSION).$(OS)-$(ARCH);\
	mv buildx-$(BUILDX_VERSION).$(OS)-$(ARCH) $(BUILDX_PLUGIN);\
	chmod a+x $(BUILDX_PLUGIN);\
	$(BUILDX_PLUGIN) create --use --platform linux/amd64,linux/ppc64le,linux/2390x;\
	}
endif

# ====================================================================================
# Targets

img.build: buildx
	@$(INFO) docker build $(IMAGE)
	@cp Dockerfile $(IMAGE_TEMP_DIR) || $(FAIL)
	@cp $(OUTPUT_DIR)/bin/$(OS)_$(ARCH)/provider $(IMAGE_TEMP_DIR) || $(FAIL)
	@cd $(IMAGE_TEMP_DIR) && $(SED_CMD) 's|BASEIMAGE|$(OSBASEIMAGE)|g' Dockerfile || $(FAIL)
	@docker buildx build $(BUILDX_ARGS) \
		$(BUILD_ARGS) \
		--build-arg ARCH=$(ARCH) \
		--build-arg TINI_VERSION=$(TINI_VERSION) \
		-t $(IMAGE) \
		$(IMAGE_TEMP_DIR) || $(FAIL)
	@$(OK) docker build $(IMAGE)