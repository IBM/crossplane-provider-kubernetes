#!/bin/bash
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

############################################################
# GKE section
############################################################
PROJECT ?= oceanic-guard-191815
ZONE    ?= us-east5-c
CLUSTER ?= bedrock-prow

activate-serviceaccount:
ifdef GOOGLE_APPLICATION_CREDENTIALS
	gcloud auth activate-service-account --key-file="$(GOOGLE_APPLICATION_CREDENTIALS)" || true
endif

get-cluster-credentials: activate-serviceaccount
	mkdir -p ~/.kube; cp -v /etc/kubeconfig/config ~/.kube; kubectl config use-context default; kubectl get nodes; echo going forward retiring google cloud
ifdef GOOGLE_APPLICATION_CREDENTIALS
	gcloud container clusters get-credentials "$(CLUSTER)" --project="$(PROJECT)" --zone="$(ZONE)" || true
endif

config-docker: get-cluster-credentials
	@ibm/scripts/config_docker.sh

############################################################
# Setup Docker buildx
############################################################

export BUILDX=$(shell docker buildx version 2>/dev/null | grep buildx)
export BUILDX_PLUGIN=$(shell pwd)/bin/docker-buildx

buildx:
ifeq (,$(BUILDX))
	@{ \
	set -e ;\
	mkdir -p bin ;\
	echo "Downloading docker-buildx ...";\
	curl -LO https://github.com/docker/buildx/releases/download/$(BUILDX_VERSION)/buildx-$(BUILDX_VERSION).$(OS)-$(ARCH);\
	mv buildx-$(BUILDX_VERSION).$(OS)-$(ARCH) $(BUILDX_PLUGIN);\
	chmod a+x $(BUILDX_PLUGIN);\
	$(BUILDX_PLUGIN) create --use --platform linux/amd64,linux/ppc64le,linux/s390x;\
	}
endif

build.init: buildx

############################################################
# Prow section
############################################################

# Specify whether this repo is build locally or not, default values is '1';
# If set to 1, then you need to also set 'DOCKER_USERNAME' and 'DOCKER_PASSWORD'
# environment variables before build the repo.
BUILD_LOCALLY ?= 1

ifeq ($(BUILD_LOCALLY),0)
DOCKER_REGISTRY = docker-na-public.artifactory.swg-devops.com/hyc-cloud-private-integration-docker-local/ibmcom
endif
export BUILD_REGISTRY=$(DOCKER_REGISTRY)

export OSBASEIMAGE=registry.access.redhat.com/ubi8/ubi-minimal:latest
IMAGE_NAME ?= ibm-crossplane-provider-kubernetes

ifeq ($(HOSTOS),darwin)
MANIFEST_TOOL_ARGS ?= --username $(DOCKER_USERNAME) --password $(DOCKER_PASSWORD)
else
MANIFEST_TOOL_ARGS ?=
endif

images: $(MANIFEST_TOOL)
ifeq ($(BUILD_LOCALLY),1)
	@$(MANIFEST_TOOL) --version

	@make build.all BUILDX_ARGS=--push
	@$(MANIFEST_TOOL) $(MANIFEST_TOOL_ARGS) push from-args --platforms linux/amd64,linux/ppc64le,linux/s390x --template $(DOCKER_REGISTRY)/$(IMAGE_NAME):$(VERSION)-ARCH --target $(DOCKER_REGISTRY)/$(IMAGE_NAME):$(VERSION) || $(FAIL)
	@$(MANIFEST_TOOL) $(MANIFEST_TOOL_ARGS) push from-args --platforms linux/amd64,linux/ppc64le,linux/s390x --template $(DOCKER_REGISTRY)/$(IMAGE_NAME):$(VERSION)-ARCH --target $(DOCKER_REGISTRY)/$(IMAGE_NAME):$(VERSION)-$(GIT_VERSION) || $(FAIL)
	@$(MANIFEST_TOOL) $(MANIFEST_TOOL_ARGS) push from-args --platforms linux/amd64,linux/ppc64le,linux/s390x --template $(DOCKER_REGISTRY)/$(IMAGE_NAME)-operator:$(VERSION)-ARCH --target $(DOCKER_REGISTRY)/$(IMAGE_NAME)-operator:$(VERSION) || $(FAIL)
	@$(MANIFEST_TOOL) $(MANIFEST_TOOL_ARGS) push from-args --platforms linux/amd64,linux/ppc64le,linux/s390x --template $(DOCKER_REGISTRY)/$(IMAGE_NAME)-operator:$(VERSION)-ARCH --target $(DOCKER_REGISTRY)/$(IMAGE_NAME)-operator:$(VERSION)-$(GIT_VERSION) || $(FAIL)
else
	@make config-docker
	@make build.all BUILDX_ARGS=--push
	@$(MANIFEST_TOOL) $(MANIFEST_TOOL_ARGS) push from-args --platforms linux/amd64,linux/ppc64le,linux/s390x --template $(DOCKER_REGISTRY)/$(IMAGE_NAME):$(VERSION)-ARCH --target $(DOCKER_REGISTRY)/$(IMAGE_NAME):$(VERSION) || $(FAIL)
	@$(MANIFEST_TOOL) $(MANIFEST_TOOL_ARGS) push from-args --platforms linux/amd64,linux/ppc64le,linux/s390x --template $(DOCKER_REGISTRY)/$(IMAGE_NAME):$(VERSION)-ARCH --target $(DOCKER_REGISTRY)/$(IMAGE_NAME):$(VERSION)-$(GIT_VERSION) || $(FAIL)
	@$(MANIFEST_TOOL) $(MANIFEST_TOOL_ARGS) push from-args --platforms linux/amd64,linux/ppc64le,linux/s390x --template $(DOCKER_REGISTRY)/$(IMAGE_NAME)-operator:$(VERSION)-ARCH --target $(DOCKER_REGISTRY)/$(IMAGE_NAME)-operator:$(VERSION) || $(FAIL)
	@$(MANIFEST_TOOL) $(MANIFEST_TOOL_ARGS) push from-args --platforms linux/amd64,linux/ppc64le,linux/s390x --template $(DOCKER_REGISTRY)/$(IMAGE_NAME)-operator:$(VERSION)-ARCH --target $(DOCKER_REGISTRY)/$(IMAGE_NAME)-operator:$(VERSION)-$(GIT_VERSION) || $(FAIL)
endif


