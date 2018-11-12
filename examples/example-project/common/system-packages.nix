{ config, pkgs, ...}:
{
  # list of packages to include on all hosts
  environment.systemPackages = with pkgs; [
    vim
    htop
  ];
}
