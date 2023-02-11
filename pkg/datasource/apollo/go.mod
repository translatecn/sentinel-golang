module github.com/alibaba/sentinel-golang/pkg/datasource/apollo

go 1.13

replace github.com/alibaba/sentinel-golang => ../../../

require (
	github.com/alibaba/sentinel-golang v1.0.3
	github.com/apolloconfig/agollo/v4 v4.0.9
	github.com/pkg/errors v0.9.1
	github.com/stretchr/testify v1.7.0
)

require (
	github.com/google/go-cmp v0.5.6 // indirect
	github.com/kr/pretty v0.2.0 // indirect
	golang.org/x/sys v0.0.0-20211117180635-dee7805ff2e1 // indirect
	google.golang.org/protobuf v1.27.1 // indirect
)
