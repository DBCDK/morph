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

    enable = mkEnableOption "Vault features";

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

    destinationFile = mkOption {
      type = submodule {

        options = {
          path = mkOption {
            type = str;
            default = "/var/secrets/vault.env";
            description = "Full path (including filename) of the remote env-file which will hold Vault credentials.";
          };

          owner = mkOption {
            type = ownerOptionsType;
            default = { user = "root"; group = "root"; };
            description = "User that will own the file containing Vault credentials on the remote host.";
          };

          permissions = mkOption {
            type = str;
            default = "0400";
            description = "Permissions expressed as octal.";
          };
        };
      };
      
      default = {};
    };
  };

});

healthCheckType = submodule ({ ... }: {
  options = {
    cmd = mkOption {
      type = listOf cmdHealthCheckType;
      default = [];
      description = "List of command health checks";
    };
    http = mkOption {
      type = listOf httpHealthCheckType;
      default = [];
      description = "List of HTTP health checks";
    };
  };
});

httpHealthCheckType = types.submodule ({ ... }: {
  options = {
    description = mkOption {
        type = str;
        description = "Health check description";
    };
    host = mkOption {
      type = nullOr str;
      description = "Host name";
      default = null;
      #default = config.networking.hostName;
    };
    scheme = mkOption {
      type = str;
      description = "Scheme";
      default = "http";
    };
    port = mkOption {
      type = int;
      description = "Port number";
    };
    path = mkOption {
      type = path;
      description = "HTTP request path";
      default = "/";
    };
    headers = mkOption {
      type = attrsOf str;
      description = "not implemented";
      default = {};
    };
    period = mkOption {
      type = int;
      description = "Seconds between checks";
      default = 2;
    };
    timeout = mkOption {
      type = int;
      description = "Timeout in seconds";
      default = 5;
    };
    insecureSSL = mkOption {
      type = bool;
      description = "Ignore SSL errors";
      default = false;
    };
  };
});

cmdHealthCheckType = types.submodule ({ ... }: {
  options = {
    description = mkOption {
        type = str;
        description = "Health check description";
    };
    cmd = mkOption {
        type = nullOr (listOf str);
        description = "Command to run as list";
        default = null;
    };
    period = mkOption {
      type = int;
      description = "Seconds between checks";
      default = 2;
    };
    timeout = mkOption {
      type = int;
      description = "Timeout in seconds";
      default = 5;
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

    healthChecks = mkOption {
      type = healthCheckType;
      description = ''
        Health check configuration.
      '';
      default = {};
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
