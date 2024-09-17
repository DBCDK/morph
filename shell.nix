{
  nixpkgs ? import ./nixpkgs.nix,
  pkgs ? import nixpkgs { },
}:

let
  morph = pkgs.callPackage ./default.nix { };

in
pkgs.mkShell { inputsFrom = [ morph ]; }
