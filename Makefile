all: dependencies assets build

dependencies:
	dep ensure -v -vendor-only

assets:
	go-bindata -pkg assets -o assets/assets.go data/

build:
	go build

derivation: dependencies
	make_derivation

.PHONY: assets
