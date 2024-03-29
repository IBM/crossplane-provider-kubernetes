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

FROM BASEIMAGE

ARG VCS_REF
ARG VCS_URL
ARG IMAGE_NAME
ARG IMAGE_DESCRIPTION
ARG ARCH_TYPE

# http://label-schema.org/rc1/
LABEL org.label-schema.vendor="IBM" \
      org.label-schema.name="$IMAGE_NAME" \
      org.label-schema.description="$IMAGE_DESCRIPTION" \
      org.label-schema.vcs-ref=$VCS_REF \
      org.label-schema.vcs-url=$VCS_URL \
      org.label-schema.license="Licensed Materials - Property of IBM" \
      org.label-schema.schema-version="1.0" \
      vendor="IBM" \
      name="$IMAGE_NAME" \
      summary="$IMAGE_DESCRIPTION" \
      description="$IMAGE_DESCRIPTION" \
      version="1.0" \
      release="$VCS_REF"

ADD provider /usr/local/bin/crossplane-kubernetes-provider

ENV XDG_CACHE_HOME /tmp

EXPOSE 8080
USER 1001
ENTRYPOINT ["crossplane-kubernetes-provider"]