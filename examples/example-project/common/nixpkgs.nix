args@{ version ? "18.09", ... }:
with builtins;
let
  pin = fromJSON (readFile (./nixpkgs- + version + ".json"));

  # Prepend the default overlay to args.overlays
  overlays = [ (import ../pkgs) ] ++ (args . overlays or []);
  nixpkgsArgs = (removeAttrs args ["version"]) // {overlays = overlays;};

  pkgs = import (fetchTarball { inherit (pin) url sha256; }) nixpkgsArgs;
in
  pkgs
