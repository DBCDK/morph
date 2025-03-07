# gomod2nix: https://github.com/nix-community/gomod2nix/blob/master/docs/getting-started.md
{
  pkgs ? (
    let
      sources = import ./sources.nix;
    in
    import sources.nixpkgs { overlays = [ (import "${sources.gomod2nix}/overlay.nix") ]; }
  ),
  version ? "dev",
}:

pkgs.buildGoApplication rec {
  name = "morph-unstable-${version}";
  inherit version;

  src = pkgs.nix-gitignore.gitignoreSource [ ] ./.;

  ldflags = [
    "-X main.version=${version}"
    "-X main.assetRoot=${placeholder "lib"}"
  ];

  modules = ./gomod2nix.toml;

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
    mainProgram = "morph";
  };
}
