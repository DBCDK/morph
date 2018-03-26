{ config, lib, pkgs, ... }:

with lib;

let

keyOptionsType = types.submodule ({ ... }: {
  options = {
    destination = mkOption {
      type = types.str;
      description = "Remote path";
    };

    source = mkOption {
      type = types.str;
      description = "Local path";
    };
  };
});

in

{
  options.deployment = {
    targetHost = mkOption {
      type = types.str;
    };
    secrets = mkOption {
      default = {};
      example = { password.text = "foobar"; };
      type = types.attrsOf keyOptionsType;
      description = ''
        Attrset where each attribute describes a key to be copied via ssh
        instead of through the Nix closure (keeping it out of the Nix store.)
      '';
    };
  };
}
