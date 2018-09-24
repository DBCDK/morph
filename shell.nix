{ pkgs ? (import <nixpkgs> {}) }:

with pkgs;
let
  dep2nix = callPackage ./nix-packaging/dep2nix {};
  packagingOut = "./nix-packaging";

  shellHook = ''
    if [[ -f ./result-bin/bin/morph ]] && [[ `which morph 2>&1 >/dev/null` ]]; then
      export PATH=$PATH:$(readlink -f ./result-bin/bin)
      source <(morph --completion-script-bash)
    fi
  '';
  makeEnv = writeShellScriptBin "make-env" shellHook;
  makeDeps = writeShellScriptBin "make-deps" ''
    set -e

    outpath=$(readlink -f ${packagingOut})
    outpath="$outpath/deps.nix"

    ${dep2nix}/bin/dep2nix -i Gopkg.lock -o $outpath
  '';
  makeBuild = writeShellScriptBin "make-build"  ''
    set -e

    outpath="$(readlink -f ${packagingOut})/deps.nix"

    ${nix}/bin/nix-build -E 'with import <nixpkgs> {};
      callPackage ./nix-packaging/default.nix {}' $@

    make-env
  '';
in
  # Change to mkShell once that hits stable!
  stdenv.mkDerivation {
    name = "morph-build-env";

    buildInputs = [
     bashInteractive
     dep
     git
     makeEnv
     makeDeps
     makeBuild
     nix-prefetch-git
    ];

    inherit shellHook;
  }
