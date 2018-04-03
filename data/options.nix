{ config, lib, pkgs, ... }:

with lib;
with lib.types;

let

ownerOptionsType = submodule ({ ... }: {
    options = {
        group = mkOption {
            type = str;
            description = "Group that will own the secret.";
            default = "root";
        };

        user = mkOption {
            type = str;
            description = "User who will own the secret.";
            default = "root";
        };
    };
});

keyOptionsType = submodule ({ ... }: {
  options = {
    destination = mkOption {
      type = str;
      description = "Remote path";
    };

    source = mkOption {
      type = str;
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
      type = str;
      description = "Permissions expressed as octal.";
    };
  };
});

vaultOptionsType = submodule ({ ... }: {

  options = {

    ttl = mkOption {
      type = str;
      default = "43200m"; # 30 days
      description = "TTL for secret tokens for this host.";
    };

    cidrs = mkOption {
      type = listOf str;
      default = [];
      example = ["172.20.11.12/32"];
      description = "IPv4 CIDR block that can login using secret tokens for this host.";
    };

    policies = mkOption {
      type = listOf str;
      default = ["default"];
      example = ["k8s" "control-plane"];
      description = "Vault access policies to apply for this host.";
    };

  };

});

in

{
  options.deployment = {
    targetHost = mkOption {
      type = str;
    };
    secrets = mkOption {
      default = {};
      example = { password.text = "foobar"; };
      type = attrsOf keyOptionsType;
      description = ''
        Attrset where each attribute describes a key to be copied via ssh
        instead of through the Nix closure (keeping it out of the Nix store.)
      '';
    };
    vault = mkOption {
      default = {};
      type = vaultOptionsType;
      description = ''
        Hashicorp Vault options for configuring approle tokens for hosts.
      '';
    };
  };
}
