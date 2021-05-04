# Intentionally impure for CI against nixos-unstable
builtins.fetchTarball "https://github.com/NixOS/nixpkgs/archive/nixos-unstable.tar.gz"
