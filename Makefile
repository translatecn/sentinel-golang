tidy:
	find .|grep 'go\.sum'|xargs -I F rm -rf F
	go mod tidy
	cd ./pkg/datasource/nacos && go mod tidy && cd -
	cd ./pkg/datasource/k8s && go mod tidy && cd -
	cd ./pkg/datasource/etcdv3 && go mod tidy && cd -
	cd ./pkg/datasource/consul && go mod tidy && cd -
	cd ./pkg/datasource/apollo && go mod tidy && cd -
	cd ./pkg/adapters/grpc && go mod tidy && cd -
	cd ./pkg/adapters/go-zero && go mod tidy && cd -
	cd ./pkg/adapters/gin && go mod tidy && cd -
