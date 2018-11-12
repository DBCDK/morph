{ config, pkgs, ...}:
{
  # use EFI by default
  boot.loader.systemd-boot.enable = true;
  boot.loader.efi.canTouchEfiVariables = true;
}
