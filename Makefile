test:
	GO111MODULE=on go test -v -count=1 -cover ./graphql .

.PHONY: test
