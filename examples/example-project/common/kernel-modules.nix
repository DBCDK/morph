{ config, pkgs, ... }:
{
  # list of kernel modules to include on all hosts. Useful for things like raid controllers.
  boot.initrd.availableKernelModules = [
  ];

  # some hardware require extra firmware (e.g. enterprise NIC's):
  # hardware.enableRedistributableFirmware = true;
}
