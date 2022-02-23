FROM hyc-cloud-private-edge-docker-local.artifactory.swg-devops.com/build-images/ubi8-minimal:latest

ADD provider /usr/local/bin/crossplane-kubernetes-provider

ENV XDG_CACHE_HOME /tmp

EXPOSE 8080
USER 1001
ENTRYPOINT ["crossplane-kubernetes-provider"]