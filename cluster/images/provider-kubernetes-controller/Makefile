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
# Setup Project

PLATFORMS := linux_amd64 linux_ppc64le linux_s390x

VCS_URL = $(shell git config --get remote.origin.url)
VCS_REF ?= $(shell git rev-parse HEAD)

include ../../../build/makelib/common.mk


# ====================================================================================
#  Options
IMAGE_NAME = provider-kubernetes-operator
IMAGE = $(BUILD_REGISTRY)/ibm-crossplane-$(IMAGE_NAME):$(VERSION)-$(ARCH)

include ../../../build/makelib/image.mk

# ====================================================================================
# Targets

img.build:
ifneq (,$(BUILDX))
	$(eval CONTAINER_CLI = docker buildx build --platform linux/$(ARCH) $(BUILDX_ARGS))
else
ifneq (,$(shell which podman))
	$(eval CONTAINER_CLI = podman build --arch $(ARCH) --format docker)
else
ifneq (,$(BUILDX_PLUGIN))
	$(eval CONTAINER_CLI = $(BUILDX_PLUGIN) build --platform linux/$(ARCH) $(BUILDX_ARGS))
else
	$(eval CONTAINER_CLI = docker build)
endif
endif
endif
	@$(INFO) $(CONTAINER_CLI) $(IMAGE)
	@cp Dockerfile $(IMAGE_TEMP_DIR) || $(FAIL)
	@cp $(OUTPUT_DIR)/bin/$(OS)_$(ARCH)/provider $(IMAGE_TEMP_DIR) || $(FAIL)
	@cd $(IMAGE_TEMP_DIR) && $(SED_CMD) 's|BASEIMAGE|$(OSBASEIMAGE)|g' Dockerfile || $(FAIL)
	@$(CONTAINER_CLI) $(BUILD_ARGS) \
		--build-arg ARCH=$(ARCH) \
		--build-arg TINI_VERSION=$(TINI_VERSION) \
		--build-arg VCS_REF=$(VCS_REF) \
		--build-arg VCS_URL=$(VCS_URL) \
		-t $(IMAGE) \
		$(IMAGE_TEMP_DIR) || $(FAIL)
	@$(OK) $(CONTAINER_CLI) $(IMAGE)