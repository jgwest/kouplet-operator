module github.com/jgwest/kouplet

go 1.13

require (
	github.com/google/uuid v1.1.1
	github.com/joshdk/go-junit v0.0.0-20200702055522-6efcf4050909
	github.com/minio/minio-go/v6 v6.0.49
	github.com/operator-framework/operator-sdk v0.18.2
	github.com/spf13/pflag v1.0.5
	k8s.io/api v0.18.2
	k8s.io/apimachinery v0.18.2
	k8s.io/client-go v12.0.0+incompatible
	sigs.k8s.io/controller-runtime v0.6.0
)

replace (
	github.com/Azure/go-autorest => github.com/Azure/go-autorest v13.3.2+incompatible // Required by OLM
	k8s.io/client-go => k8s.io/client-go v0.18.2 // Required by prometheus-operator
)
