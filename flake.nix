{
  description = "Morph: NixOS deployment tool";

  inputs = {
    flake-utils.url = "github:numtide/flake-utils";
    nixpkgs.url = "nixpkgs/nixos-unstable";
    flake-compat = { url = "github:edolstra/flake-compat"; flake = false; };
  };

  outputs =
    inputs@{ self
    , nixpkgs
    , flake-utils
    , flake-compat
    }:
    { }
    //
    (flake-utils.lib.eachDefaultSystem
      (system:
      let
        pkgs = import nixpkgs
          {
            inherit system;
            overlays = [
              self.overlay
            ];
          };
      in
      rec {
        devShell = import ./shell.nix { inherit pkgs; };
        defaultPackage = pkgs.morph;
        packages = {
          inherit (pkgs)
            morph;
        };
        hydraJobs = {
          inherit packages;
        };
      }
      )
    ) //
    {
      overlay = final: prev: with prev;
        {
          morph = callPackage ./. { };
        };
    };
}
