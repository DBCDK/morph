{ pkgs ? (import <nixpkgs> {}) }:

with pkgs;
let
  dep2nix = callPackage ./nix-packaging/dep2nix {};
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
      ${dep}/bin/dep ensure -v -update
    else
      ${dep}/bin/dep ensure -v -vendor-only
    fi

    # Write /nix-packaging/deps.nix (for use in distribution)
    outpath=$(readlink -f ${packagingOut})
    outpath="$outpath/deps.nix"

    ${dep2nix}/bin/dep2nix -i Gopkg.lock -o $outpath
  '';
  makeBuild = writeShellScriptBin "make-build"  ''
    set -e

    outpath="$(readlink -f ${packagingOut})/deps.nix"

    ${nix}/bin/nix-build -E 'with import <nixpkgs> {};
      callPackage ./nix-packaging/default.nix {}' -A bin $@

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
