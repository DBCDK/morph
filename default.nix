{ nixpkgs ? import ./nixpkgs.nix
, pkgs ? import nixpkgs {}
, version ? "dev"
}:

let
  morph = pkgs.buildGoModule rec {
    name = "morph-unstable-${version}";
    inherit version;

    nativeBuildInputs = with pkgs; [ go-bindata ];

    src = pkgs.nix-gitignore.gitignoreSource [] ./.;

    buildFlagsArray = ''
      -ldflags=
      -X
      main.version=${version}
    '';

    vendorSha256 = "08zzp0h4c4i5hk4whz06a3da7qjms6lr36596vxz0d8q0n7rspr9";

    postPatch = ''
      go-bindata -pkg assets -o assets/assets.go data/
    '';

    postInstall = ''
      mkdir -p $lib
      cp -v ./data/*.nix $lib
    '';

    outputs = [ "out" "lib" ];

    passthru = {
      eval = args@{...}: (import (morph.lib + "/eval-machines.nix")) ({ inherit pkgs; } // args);
    };

    meta = {
      homepage = "https://github.com/DBCDK/morph";
      description = "Morph is a NixOS host manager written in Golang.";
    };
  };
in morph
