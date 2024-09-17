let
  pkgs = import (import ../nixpkgs.nix) { };
in
{
  network = {
    inherit pkgs;
    description = "webserver with secrets";
  };

  "web01" = _: {
    deployment = {
      secrets = {
        "nix-cache-signing-key" = {
          source = "../secrets/very-secret.txt";
          destination = "/var/secrets/very-secret.txt";
          owner.user = "nginx";
          owner.group = "root";
          permissions = "0400"; # this is the default
          action = [
            "sudo"
            "systemctl"
            "reload"
            "nginx.service"
          ];
        };
      };
    };

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
  };
}
