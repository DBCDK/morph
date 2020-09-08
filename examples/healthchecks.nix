let
  # Pin the deployment package-set to a specific version of nixpkgs
  pkgs = import (builtins.fetchTarball {
    url = "https://github.com/NixOS/nixpkgs-channels/archive/51d115ac89d676345b05a0694b23bd2691bf708a.tar.gz";
    sha256 = "1gfjaa25nq4vprs13h30wasjxh79i67jj28v54lkj4ilqjhgh2rs";
  }) {};
in
{
  network =  {
    inherit pkgs;
    description = "health check demo hosts";
  };

  "web01.example.com" = { config, pkgs, ... }: {
    boot.loader.systemd-boot.enable = true;
    boot.loader.efi.canTouchEfiVariables = true;

    services.nginx.enable = true;

    fileSystems = {
        "/" = { label = "nixos"; fsType = "ext4"; };
        "/boot" = { label = "boot"; fsType = "vfat"; };
    };

    deployment = {
      healthChecks = {
        cmd = [{
          cmd = ["true" "one argument" "another argument"];
          description = "Testing that 'true' works.";
        }];

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
    };
  };
}
