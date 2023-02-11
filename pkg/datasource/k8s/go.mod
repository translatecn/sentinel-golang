module github.com/alibaba/sentinel-golang/pkg/datasource/k8s

go 1.13

require (
	github.com/alibaba/sentinel-golang v1.0.2
	github.com/go-logr/logr v0.1.0
	github.com/pkg/errors v0.9.1
	github.com/stretchr/testify v1.6.1
	k8s.io/apimachinery v0.17.2
	k8s.io/client-go v0.17.2
	sigs.k8s.io/controller-runtime v0.5.0
)

replace github.com/alibaba/sentinel-golang v1.0.2 => ../../../
