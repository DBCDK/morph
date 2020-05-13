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

  goPackagePath = "github.com/dbcdk/morph";

  nativeBuildInputs = [ go-bindata ];

  src = filterSource srcFilter ./..;
  goDeps = ./deps.nix;

  buildFlagsArray = ''
    -ldflags=
    -X
    main.version=${version}
  '';

  postPatch = ''
    go-bindata -pkg assets -o assets/assets.go data/
  '';

  postInstall = ''
    mkdir -p $lib
    cp -v $src/data/*.nix $lib
  '';

  outputs = [ "out" "lib" ];

  meta = {
    homepage = "https://github.com/dbcdk/morph";
    description = "Morph is a NixOS host manager written in Golang.";
  };
}
