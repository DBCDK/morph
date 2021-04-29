{ stdenv, fetchgit, buildGoModule, go-bindata, lib
, version ? "dev"
}:

buildGoModule rec {
  name = "morph-unstable-${version}";
  inherit version;

  nativeBuildInputs = [ go-bindata ];

  src = import ./source.nix { inherit lib; };

  buildFlagsArray = ''
    -ldflags=
    -X
    main.version=${version}
  '';

  vendorSha256 = "05rfvbqicr1ww4fjf6r1l8fb4f0rsv10vxndqny8wvng2j1rmmm6";

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
