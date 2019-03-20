{ nixpkgs ? builtins.fetchTarball "https://github.com/NixOS/nixpkgs/archive/nixos-unstable.tar.gz"
, pkgs ? import nixpkgs {}
}:

with pkgs;
let
  packagingOut = "./nix-packaging";

  shellHook = ''
    if [[ -f ./result/bin/morph ]]; then
      if [[ `${which} morph 2>&1 >/dev/null` ]]; then
        export PATH=$PATH:$(pwd)/result/bin
      fi
      source <(morph --completion-script-bash)
    fi
  '';
  makeEnv = writeScriptBin "make-env" (''
    #!${bashInteractive}/bin/bash
  '' + shellHook);
  makeDeps = writeShellScriptBin "make-deps" ''
    set -e

    # Populate /vendor-dir (for convenience in local dev)
    if [ "$1" == "update" ]; then
      ${go}/bin/go get -u

      # compute the sha256 of the dependencies
      GO111MODULE=on GOPATH="$TMPDIR/gopath" ${go}/bin/go mod download
      sha256="$(${nix}/bin/nix hash-path --base32 "$TMPDIR/gopath/pkg/mod/cache/download" | tr -d '\n')"
      sed -e "s#modSha256.*#modSha256 = \"$sha256\";#" -i ${packagingOut}/default.nix
    else
      ${go}/bin/go mod vendor
    fi
  '';
  makeBuild = writeShellScriptBin "make-build" ''
    set -e

    ${nix}/bin/nix-build -E 'with import ${nixpkgs} {};
      callPackage ./nix-packaging/default.nix {}' -A out $@

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
