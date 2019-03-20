{ stdenv, fetchgit, buildGoModule, go-bindata, lib
, version ? "dev"
}:

with builtins; with lib;
let
  blacklistedDirs = [ "nix-packaging" "vendor" "^\\..+$" ];
  whitelistedFiles = [ "^.+\\.nix$" "^.+\\.go$" "^.+\\.mod$" "^.+\\.sum$"];
  filterList = file: list: elem true (map (pattern: isList (match pattern file)) list);
  srcFilter = path: type: (
    if type == "regular" then filterList (baseNameOf path) whitelistedFiles
    else if type == "directory" then !filterList (baseNameOf path) blacklistedDirs
    else false);

in
buildGoModule rec {
  name = "morph-unstable-${version}";
  inherit version;

  nativeBuildInputs = [ go-bindata ];

  src = filterSource srcFilter ./..;

  buildFlagsArray = ''
    -ldflags=
    -X
    main.version=${version}
  '';

  modSha256 = "0kwwvd979zhdml3shw96cwyh84qn7k7p4yy0qsjiwi9ncnjb1ca6";

  postPatch = ''
    go-bindata -pkg assets -o assets/assets.go data/
  '';

  postInstall = ''
    mkdir -p $lib
    cp -v go/src/$goPackagePath/data/*.nix $lib
  '';

  outputs = [ "out" "lib" ];

  meta = {
    homepage = "https://github.com/DBCDK/morph";
    description = "Morph is a NixOS host manager written in Golang.";
  };
}
