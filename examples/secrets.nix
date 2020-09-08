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
    description = "webserver with secrets";
  };

  "web01.example.com" = { config, pkgs, ... }: {
    deployment = {
      secrets = {
        "nix-cache-signing-key" = {
          source = "../secrets/very-secret.txt";
          destination = "/var/secrets/very-secret.txt";
          owner.user = "nginx";
          owner.group = "root";
          permissions = "0400"; # this is the default
          action = ["sudo" "systemctl" "reload" "nginx.service"];
        };
      };
    };

    boot.loader.systemd-boot.enable = true;
    boot.loader.efi.canTouchEfiVariables = true;

    services.nginx.enable = true;

    fileSystems = {
        "/" = { label = "nixos"; fsType = "ext4"; };
        "/boot" = { label = "boot"; fsType = "vfat"; };
    };
  };
}
