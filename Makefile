all: dependencies assets build

git_rev := $(shell git rev-parse HEAD)

dependencies:
	dep ensure -v -vendor-only

assets:
	go-bindata -pkg assets -o assets/assets.go data/

build:
	go build

derivation: dependencies
	mkdir -p nix-packaging/out
	sed -e s/__VERSION__/$(git_rev)/g nix-packaging/template/default.nix > nix-packaging/out/default.nix
	dep2nix -i Gopkg.lock -o nix-packaging/out/deps.nix

.PHONY: assets
