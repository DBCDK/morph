let
  # Pin the deployment package-set to a specific version of nixpkgs
  pkgs = import (builtins.fetchTarball {
    url = "https://github.com/NixOS/nixpkgs-channels/archive/98c1150f2cc62b94b693dce63adc1fbcbfe616f1.tar.gz";
    sha256 = "1mdwn0qrjc8jli8cbi4cfkar6xq15l232r371p4b48v2d4bah3wp";
  }) {};
in
{
  network =  {
    inherit pkgs;
    description = "simple hosts";
  };

  "web01.example.com" = { config, pkgs, ... }: {
    boot.loader.systemd-boot.enable = true;
    boot.loader.efi.canTouchEfiVariables = true;

    services.nginx.enable = true;

    fileSystems = {
        "/" = { label = "nixos"; fsType = "ext4"; };
        "/boot" = { label = "boot"; fsType = "vfat"; };
    };
  };

  "db01.example.com" = { config, pkgs, ... }: {
    boot.loader.systemd-boot.enable = true;
    boot.loader.efi.canTouchEfiVariables = true;

    services.postgresql.enable = true;

    fileSystems = {
        "/" = { label = "nixos"; fsType = "ext4"; };
        "/boot" = { label = "boot"; fsType = "vfat"; };
    };
  };
}
