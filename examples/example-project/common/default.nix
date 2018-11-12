{ config, pkgs, ...}:
with builtins;
{
  imports = [
    ./efi.nix
    ./filesystems.nix
    ./kernel-modules.nix
    ./modules.nix
    ./nix.nix
    ./system-packages.nix
    ./users
  ];

  system.stateVersion = "18.09";

  # Basic health check useful for mosts hosts
  deployment = {
    healthChecks.cmd = [{
      cmd = [(toString (pkgs.writeScript "systemd-unit-healthcheck" ''
        test $(systemctl status --no-legend --no-pager | egrep -c "\s*Jobs: 0 queued\s*") -eq 1
        test $(systemctl status --no-legend --no-pager | egrep -c "^\s*Failed: 0 units\s*$") -eq 1
      ''))];
      description = "Ask systemd whether there are failed or queued units.";
    }];
  };

  programs.bash.enableCompletion = true;
}
