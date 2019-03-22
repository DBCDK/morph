{ stdenv, fetchgit, buildGoModule, go-bindata, lib
, version ? "dev"
}:

buildGoModule rec {
  name = "morph-unstable-${version}";
  inherit version;

  buildInputs = [ go-bindata ];

  src = import ./source.nix { inherit lib; };

  buildFlagsArray = ''
    -ldflags=
    -X
    main.version=${version}
  '';

  modSha256 = "0kwwvd979zhdml3shw96cwyh84qn7k7p4yy0qsjiwi9ncnjb1ca6";

  prePatch = ''
    go-bindata -pkg assets -o assets/assets.go data/
  '';

  postInstall = ''
    mkdir -p $lib
    cp -v $src/data/*.nix $lib
  '';

  outputs = [ "out" "lib" ];

  meta = {
    homepage = "https://github.com/DBCDK/morph";
    description = "Morph is a NixOS host manager written in Golang.";
  };
}
