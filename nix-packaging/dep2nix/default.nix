{ pkgs, stdenv, buildGoPackage, ... }:
with import <nixpkgs>{};

buildGoPackage rec {
  name = "dep2nix-${version}";
  version = "d94a118a9f8ae90cb4831f200cd66ff3d9deffab";

  goPackagePath = "github.com/nixcloud/dep2nix";

  src = builtins.fetchGit {
    url = "https://github.com/nixcloud/dep2nix.git";
    ref = "master";
    rev = version;
  };

  goDeps = ./deps.nix;

  meta = with stdenv.lib; {
    description = "Convert `Gopkg.lock` files from golang dep into `deps.nix`";
    license = licenses.bsd3;
    homepage = https://github.com/nixcloud.io/dep2nix;
  };
}
