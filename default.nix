{ nixpkgs ? import ./nixpkgs.nix
, pkgs ? import nixpkgs {}
, version ? "dev"
}:

pkgs.buildGoModule rec {
  name = "morph-unstable-${version}";
  inherit version;

  src = pkgs.nix-gitignore.gitignoreSource [] ./.;

  ldflags = [
    "-X main.version=${version}"
  ];
  buildFlags = "-tags installed_data";
  preBuild = ''
    buildFlagsArray+=("-ldflags=-X github.com/DBCDK/morph/assets.root=$lib")
  '';

  vendorSha256 = "08zzp0h4c4i5hk4whz06a3da7qjms6lr36596vxz0d8q0n7rspr9";

  postInstall = ''
    mkdir -p $lib
    cp -v ./data/*.nix $lib
  '';

  outputs = [ "out" "lib" ];

  meta = {
    homepage = "https://github.com/DBCDK/morph";
    description = "Morph is a NixOS host manager written in Golang.";
  };
}
