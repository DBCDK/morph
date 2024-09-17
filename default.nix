{
  nixpkgs ? import ./nixpkgs.nix,
  pkgs ? import nixpkgs { },
  version ? "dev",
}:

pkgs.buildGoModule rec {
  name = "morph-unstable-${version}";
  inherit version;

  src = pkgs.nix-gitignore.gitignoreSource [ ] ./.;

  ldflags = [
    "-X main.version=${version}"
    "-X main.assetRoot=${placeholder "lib"}"
  ];

  vendorHash = "sha256-zQlMtbXgrH83zrcIoOuFhb2tYCeQ1pz4UQUvRIsLMCE==";

  postInstall = ''
    mkdir -p $lib
    cp -v ./data/*.nix $lib
  '';

  outputs = [
    "out"
    "lib"
  ];

  meta = {
    homepage = "https://github.com/DBCDK/morph";
    description = "Morph is a NixOS host manager written in Golang.";
  };
}
