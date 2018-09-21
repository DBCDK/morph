{ stdenv, fetchgit, buildGoPackage, go-bindata, lib,
  version ? "dev"
}:

buildGoPackage rec {
  name = "morph-unstable-${version}";

  goPackagePath = "git-platform.dbc.dk/platform/morph";

  buildInputs = [ go-bindata ];

  src = with lib; builtins.filterSource
      (path: type:
        (type == "directory" && path != "nix-packaging" && !hasPrefix "." (baseNameOf path)) ||
        (hasSuffix "data" (dirOf path) && hasSuffix ".nix" path) ||
        hasSuffix ".go" path)
        ./..;

  inherit version;

  goDeps = ./deps.nix;

  prePatch = ''
    go-bindata -pkg assets -o assets/assets.go data/
  '';

  meta = {
    homepage = "https://git-platform.dbc.dk/platform/morph";
    description = "Morph is a NixOS host manager written in GOLANG inspired the Haskell nixdeploy project.";
  };
}
