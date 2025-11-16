module github.com/weaviate/weaviate/operator

go 1.21

require (
	k8s.io/api v0.28.4
	k8s.io/apimachinery v0.28.4
	k8s.io/client-go v0.28.4
	sigs.k8s.io/controller-runtime v0.16.3
)

require (
	github.com/go-logr/logr v1.3.0
	github.com/onsi/ginkgo/v2 v2.13.0
	github.com/onsi/gomega v1.30.0
)
