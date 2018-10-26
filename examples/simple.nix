{
  network =  {
    pkgs = import <nixpkgs> {};
    description = "New host probers";
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
