{ pkgs ? (import <nixpkgs> {}) }:

let
  dep2nix = pkgs.callPackage ./nix-packaging/dep2nix {};
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
      nix-prefetch-git
    ];

    shellHook = ''
      PATH="$(pwd):$PATH"
      source <(./morph --completion-script-bash)
    '';
  }
