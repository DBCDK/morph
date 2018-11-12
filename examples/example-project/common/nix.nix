{ config, pkgs, ...}:
{
  nix = {
    trustedUsers = [ "root" "@wheel"];
    gc = {
      automatic = true;
      dates = "03:15";
      options = "--delete-older-than 60d";
    };
  };

  # Enforce NIX_PATH on all machines to use the pinned pkgs set
  environment.variables = {
    NIX_PATH = pkgs.lib.mkForce "nixpkgs=${pkgs.path}";
  };
}

