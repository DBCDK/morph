{ stdenv, fetchgit, buildGoPackage, go-bindata, lib,
  version ? "dev"
}:

with builtins; with lib;
let
  blacklistedDirs = [ "nix-packaging" "vendor" "^\\..+$" ];
  whitelistedFiles = [ "^.+\\.nix$" "^.+\\.go$" ];
  filterList = file: list: elem true (map (pattern: isList (match pattern file)) list);
  srcFilter = path: type: (
    if type == "regular" then filterList (baseNameOf path) whitelistedFiles
    else if type == "directory" then !filterList (baseNameOf path) blacklistedDirs
    else false);
in
buildGoPackage rec {
  name = "morph-unstable-${version}";
  inherit version;

  goPackagePath = "git-platform.dbc.dk/platform/morph";

  buildInputs = [ go-bindata ];

  src = filterSource srcFilter ./..;
  goDeps = ./deps.nix;

  prePatch = ''
    go-bindata -pkg assets -o assets/assets.go data/
  '';

  postInstall = ''
    mkdir -p $lib
    cp -v $src/data/*.nix $lib
  '';

  outputs = [ "out" "bin" "lib" ];

  meta = {
    homepage = "https://git-platform.dbc.dk/platform/morph";
    description = "Morph is a NixOS host manager written in GOLANG inspired the Haskell nixdeploy project.";
  };
}
