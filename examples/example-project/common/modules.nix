{ ... }:
{
  # Disable module definitions from nixpkgs, to prevent namespace collisions when overriding
  # modules locally
  disabledModules = [
    # "services/misc/gitlab.nix"
  ];

  # Import all custom modules
  imports = import ../modules/module-list.nix;
}
