{ stdenv, buildGoPackage, go-bindata }:

buildGoPackage rec {
  name = "morph-unstable-${version}";
  version = "__VERSION__";

  goPackagePath = "git-platform.dbc.dk/platform/morph";

  buildInputs = [ go-bindata ];

  src = builtins.fetchGit {
    url = "https://git-platform.dbc.dk/platform/morph.git";
    ref = "master";
    rev = version;
  };

  goDeps = ./deps.nix;

  prePatch = ''
    go-bindata -pkg assets -o assets/assets.go data/
  '';

  meta = {
    homepage = "https://git-platform.dbc.dk/platform/morph";
    description = "Morph is a NixOS host manager written in GOLANG inspired the Haskell nixdeploy project.";
  };
}
