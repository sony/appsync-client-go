test: vendor
	go test -v -cover ./graphql .

vendor:
	dep ensure

deps:
	go get -u github.com/golang/dep/cmd/dep
