{ config, lib, pkgs, ... }:

with lib;

let

ownerOptionsType = types.submodule({ ... }: {
    options = {
        group = mkOption {
            type = types.str;
            description = "Group that will own the secret.";
            default = "root";
        };

        user = mkOption {
            type = types.str;
            description = "User who will own the secret.";
            default = "root";
        };
    };
});

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

    owner = mkOption {
      default = {};
      type = ownerOptionsType;
      description = ''
        Owner of the secret.
      '';
    };

    permissions = mkOption {
      default = "0400";
      type = types.str;
      description = "Permissions expressed as octal.";
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
