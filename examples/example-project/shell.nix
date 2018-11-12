{ pkgs ? import common/nixpkgs.nix { version = "18.09"; } }:

with pkgs;
pkgs.stdenv.mkDerivation {
  name = "morph-env";

  buildInputs =
  [
    bashInteractive
    curl
    git
    jq
    morph
    nix-prefetch-git
    openssh
    rsync
  ];

  shellHook = ''
    export NIXPKGS_ALLOW_UNFREE=1
    source <(morph --completion-script-bash)
  '';
}
