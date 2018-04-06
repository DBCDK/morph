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

healthCheckType = types.submodule ({ ... }: {
  options = {
    description = mkOption {
        type = types.str;
        description = "Health check description";
    };
    host = mkOption {
      type = types.nullOr types.str;
      description = "Host name";
      default = null;
      #default = config.networking.hostName;
    };
    scheme = mkOption {
      type = types.str;
      description = "Scheme";
      default = "http";
    };
    port = mkOption {
      type = types.int;
      description = "Port number";
    };
    path = mkOption {
      type = types.path;
      description = "HTTP request path";
      default = "/";
    };
    headers = mkOption {
      type = types.attrsOf types.str;
      description = "not implemented";
      default = {};
    };
    period = mkOption {
      type = types.int;
      description = "Seconds between checks";
      default = 2;
    };
    timeout = mkOption {
      type = types.int;
      description = "Timeout in seconds";
      default = 5;
    };
    insecureSSL = mkOption {
      type = types.bool;
      description = "Ignore SSL errors";
      default = false;
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
    healthChecks = mkOption {
      type = types.listOf healthCheckType;
      default = [];
      description = ''
        List of health checks.
      '';
    };
  };
}
