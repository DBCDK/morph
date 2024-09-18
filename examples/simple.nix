let
  pkgs = import (import ../nixpkgs.nix) { };
in
{
  network = {
    inherit pkgs;
    specialArgs = {
      systemdBoot = true;
    };
    description = "simple hosts";
    ordering = {
      tags = [
        "db"
        "web"
      ];
    };
  };

  "web01" =
    { systemdBoot, ... }:
    {
      deployment.tags = [ "web" ];

      boot.loader.systemd-boot.enable = systemdBoot;
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
    };

  "db01" = _: {
    deployment.tags = [ "db" ];

    boot.loader.systemd-boot.enable = true;
    boot.loader.efi.canTouchEfiVariables = true;

    services.postgresql.enable = true;

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
  };
}
