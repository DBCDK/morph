{ pkgs ? (import <nixpkgs> {}) }:

let
  go2nix_v2 = pkgs.callPackage ./go2nix.nix {};
in
  # Change to mkShell once that hits stable!
  pkgs.stdenv.mkDerivation {
    name = "morph-build-env";

    buildInputs = with pkgs; [
      bashInteractive
      dep
      gnumake
      go-bindata
      go2nix_v2
      nix-prefetch-git
    ];

    shellHook = ''
      PATH="$(pwd):$PATH"
      source <(./morph --completion-script-bash)
    '';
  }
