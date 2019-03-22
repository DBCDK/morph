let
  nixpkgsSrc = builtins.fetchTarball {
    url = "https://github.com/NixOS/nixpkgs/archive/52dae14f0c763dd48572058f0f0906166da14c31.tar.gz";
    sha256 = "13bnnf4w3jm3cbny2hghrafblbxgxccalc12bpy141vkx2f4qb5a";
  };
in

{ pkgs ? (import nixpkgsSrc {}) }:

with pkgs;
let
  packagingOut = "./nix-packaging";

  shellHook = ''
    if [[ -f ./result-bin/bin/morph ]]; then
      if [[ `${which} morph 2>&1 >/dev/null` ]]; then
        export PATH=$PATH:$(pwd)/result-bin/bin
      fi
      source <(morph --completion-script-bash)
    fi
  '';
  makeEnv = writeScriptBin "make-env" (''
    #!${bashInteractive}/bin/bash
  '' + shellHook);
  makeDeps = writeShellScriptBin "make-deps" ''
    set -e

    export GO111MODULE=on

    # update the modules if requested
    if [ "$1" == "update" ]; then
      ${go}/bin/go get -u

      source="$( nix eval --raw '(with import ${nixpkgsSrc} {}; import ./nix-packaging/source.nix { inherit lib; })' )"

      # compute the sha256 of the dependencies
      pushd "$source" >/dev/null
        export GOPATH="$(mktemp -d)" GOCACHE="$(mktemp -d)"
        ${go}/bin/go mod download
        sha256="$( ${nix}/bin/nix hash-path --base32 "$GOPATH/pkg/mod/cache/download" | tr -d '\n' )"
      popd >/dev/null

      # replace the sha256 in the default.nix
      sed -e "s#modSha256.*#modSha256 = \"$sha256\";#" -i ${packagingOut}/default.nix

      unset GOPATH GOCACHE
    fi

    # Populate /vendor (for convenience in local dev)
    ${go}/bin/go mod vendor

    unset GO111MODULE
  '';
  makeBuild = writeShellScriptBin "make-build" ''
    set -e

    ${nix}/bin/nix build '(with import ${nixpkgsSrc} {};
      callPackage ./nix-packaging/default.nix {})' $@

    make-env
  '';
in
  # Change to mkShell once that hits stable!
  stdenv.mkDerivation {
    name = "morph-build-env";

    buildInputs = [
     bashInteractive
     git
     makeEnv
     makeDeps
     makeBuild
     nix
     nix-prefetch-git
     openssh
    ];

    inherit shellHook;
  }
