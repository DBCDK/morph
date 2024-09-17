let
  pkgs = import (import ../nixpkgs.nix) { };
in
{
  network = {
    inherit pkgs;
    description = "health check demo hosts";
  };

  "web01" = _: {
    boot.loader.systemd-boot.enable = true;
    boot.loader.efi.canTouchEfiVariables = true;

    services.nginx.enable = true;

    fileSystems = {
      "/" = {
        label = "nixos";
        fsType = "ext4";
      };
      "/boot" = {
        label = "boot";
        fsType = "vfat";
      };
    };

    deployment = {
      healthChecks = {
        cmd = [
          {
            cmd = [
              "true"
              "one argument"
              "another argument"
            ];
            description = "Testing that 'true' works.";
          }
        ];

        http = [
          {
            scheme = "http";
            port = 80;
            path = "/";
            description = "Check whether nginx is running.";
            period = 1; # number of seconds between retries
          }
          {
            scheme = "https";
            port = 443;
            host = "some-other-host.example.com"; # defaults to the hostname of the host if unset
            path = "/health";
            description = "Check whether $imaginaryService is running.";
          }
        ];
      };

      preDeployChecks = {
        # Works exactly like health checks
        # Have you read the warning about this feature in the README?
      };
    };
  };
}
