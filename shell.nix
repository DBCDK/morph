{ pkgs ? (import <nixpkgs> {}) }:

let
  dep2nix = pkgs.callPackage ./nix-packaging/dep2nix {};
  mkDerivation = pkgs.writeShellScriptBin "make_derivation" (builtins.readFile ./nix-packaging/make_derivation.sh);
in
  # Change to mkShell once that hits stable!
  pkgs.stdenv.mkDerivation {
    name = "morph-build-env";

    buildInputs = with pkgs; [
      bashInteractive
      dep
      dep2nix
      git
      gnumake
      go-bindata
      mkDerivation
      nix-prefetch-git
    ];

    shellHook = ''
      PATH="$(pwd):$PATH"
      source <(./morph --completion-script-bash)
    '';
  }
