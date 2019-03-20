let
  nixpkgsSrc = builtins.fetchTarball {
    url = "https://github.com/NixOS/nixpkgs-channels/archive/832eb2559d4a4280a0cc3e539080993e121e8d98.tar.gz";
    sha256 = "01r9w24w34nh90cjanah04fgs9nh87r2jdhmki0zkx2n4ppfnjld";
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

    ${nix}/bin/nix-build -E 'with import ${nixpkgsSrc} {};
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
