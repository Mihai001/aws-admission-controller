module github.com/giantswarm/aws-admission-controller/v2

go 1.15

require (
	github.com/blang/semver v3.5.1+incompatible
	github.com/dylanmei/iso8601 v0.1.0
	github.com/dyson/certman v0.2.1
	github.com/giantswarm/apiextensions/v3 v3.15.1
	github.com/giantswarm/backoff v0.2.0
	github.com/giantswarm/k8sclient/v5 v5.0.0
	github.com/giantswarm/microerror v0.3.0
	github.com/giantswarm/micrologger v0.5.0
	github.com/giantswarm/ruleengine v0.2.0
	github.com/giantswarm/to v0.3.0
	github.com/prometheus/client_golang v1.9.0
	github.com/stretchr/testify v1.6.1 // indirect
	golang.org/x/tools v0.0.0-20200706234117-b22de6825cf7 // indirect
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	k8s.io/api v0.18.9
	k8s.io/apiextensions-apiserver v0.18.9
	k8s.io/apimachinery v0.18.9
	k8s.io/client-go v0.18.9
	sigs.k8s.io/cluster-api v0.3.11
	sigs.k8s.io/controller-runtime v0.6.4
)

replace sigs.k8s.io/cluster-api v0.3.11 => github.com/giantswarm/cluster-api v0.3.10-gs
