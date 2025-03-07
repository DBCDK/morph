# Intentionally impure for CI against nixos-unstable
{
  nixpkgs = builtins.fetchTarball "https://github.com/NixOS/nixpkgs/archive/nixos-unstable.tar.gz";

  gomod2nix = builtins.fetchGit {
    url = "https://github.com/nix-community/gomod2nix.git";
    rev = "514283ec89c39ad0079ff2f3b1437404e4cba608";
  };
}
