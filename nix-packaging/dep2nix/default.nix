{ stdenv, fetchgit, buildGoPackage, ... }:

buildGoPackage rec {
  name = "dep2nix-${version}";
  version = "0.0.2";

  goPackagePath = "github.com/nixcloud/dep2nix";

  src = fetchgit {
    url = "https://github.com/nixcloud/dep2nix.git";
    rev = version;
    sha256 = "17csgnd6imr1l0gpirsvr5qg7z0mpzxj211p2nwqilrvbp8zj7vg";
  };

  goDeps = ./deps.nix;

  meta = with stdenv.lib; {
    description = "Convert `Gopkg.lock` files from golang dep into `deps.nix`";
    license = licenses.bsd3;
    homepage = https://github.com/nixcloud.io/dep2nix;
  };
}
