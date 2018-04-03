{ pkgs ? (import <nixpkgs> {}) }:

let
  go2nix_v2 = pkgs.callPackage ./go2nix.nix {};
in
  # Change to mkShell once that hits stable!
  pkgs.stdenv.mkDerivation {
    name = "morph-build-env";

    buildInputs = with pkgs; [
      go2nix_v2
      go-bindata
      nix-prefetch-git
      dep
      bashInteractive
    ];

    shellHook = ''
      PATH="$(pwd):$PATH"
      source <(./morph --completion-script-bash)
    '';
  }
