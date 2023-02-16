module kubebuilder

go 1.19

require (
	github.com/onsi/ginkgo/v2 v2.6.0
	github.com/onsi/gomega v1.24.1
	k8s.io/apimachinery v0.26.0
	k8s.io/client-go v0.26.0
	sigs.k8s.io/controller-runtime v0.14.1
)

require k8s.io/api v0.26.0
