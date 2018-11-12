{ config, pkgs, ...}:
{
  # default filesystems on all hosts
  fileSystems = {
      "/" = { label = "nixos"; fsType = "ext4"; };
      "/boot" = { label = "boot"; fsType = "vfat"; };
  };
}
