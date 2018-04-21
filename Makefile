
all: dependencies assets build

dependencies:
	dep ensure -vendor-only

assets:
	go-bindata -pkg assets -o assets/assets.go data/

build:
	go build

derivation: dependencies
	go2nix save

.PHONY: assets
