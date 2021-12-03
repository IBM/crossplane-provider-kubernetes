module github.com/crossplane-contrib/provider-kubernetes

go 1.16

require (
	github.com/crossplane/crossplane-runtime v0.15.0
	github.com/crossplane/crossplane-tools v0.0.0-20201201125637-9ddc70edfd0d
	github.com/google/go-cmp v0.5.5
	github.com/pkg/errors v0.9.1
	github.com/prometheus/common v0.26.0
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	k8s.io/api v0.21.2
	k8s.io/apimachinery v0.21.2
	k8s.io/client-go v0.21.2
	sigs.k8s.io/controller-runtime v0.9.2
	sigs.k8s.io/controller-tools v0.3.0
)
