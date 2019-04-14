let
  # Pin the deployment package-set to a specific version of nixpkgs
  oldPkgs = import (builtins.fetchTarball {
    url = "https://github.com/NixOS/nixpkgs-channels/archive/98c1150f2cc62b94b693dce63adc1fbcbfe616f1.tar.gz";
    sha256 = "1mdwn0qrjc8jli8cbi4cfkar6xq15l232r371p4b48v2d4bah3wp";
  }) {};

  #sysPkgs = import <nixpkgs> {};

  newPkgs = import (builtins.fetchTarball {
    url = "https://github.com/NixOS/nixpkgs-channels/archive/180aa21259b666c6b7850aee00c5871c89c0d939.tar.gz";
    sha256 = "0gxd10djy6khbjb012s9fl3lpjzqaknfv2g4dpfjxwwj9cbkj04h";
  }) {};

  vpsfPkgs = builtins.fetchTarball {
    url = "https://github.com/vpsfreecz/nixpkgs/archive/5dd15a4181fb260d1c006c4d00e4cc978cd89989.tar.gz";
    sha256 = "0yg9059n08469mndvpq1f5x3lcnj9zrynkckwh9pii1ihimj6xyl";
  };

  vpsadminos = builtins.fetchTarball {
    url = "https://github.com/vpsfreecz/vpsadminos/archive/c00b238f4d290c8eded24ca3d0ae97c320bded91.tar.gz";
    sha256 = "10m9sc49gz5j71xwm65pdw4wz683w37csi5zjfrs1jxdgy70j0pd";
  };

in
{
  network =  {
    pkgs = newPkgs;
    description = "simple hosts";
  };

  # uses network.pkgs
  "default_pkgs" = { config, pkgs, ... }: {
    boot.isContainer = true;
  };

  # uses vpsfPkgs and vpsadminos
  "vpsadminos" = { config, pkgs, ... }: {
    boot.zfs.pools = {
      tank = {
      };
    };

    deployment = {
      nixPath = [
        { prefix = "nixpkgs"; path = vpsfPkgs; }
        { prefix = "vpsadminos"; path = vpsadminos; }
      ];
      importPath = "${vpsadminos}/os/default.nix";
    };
  };

  "custom" = { config, pkgs, ... }: {
    boot.isContainer = true;

    deployment = {
      nixPath = [
        { prefix = "nixpkgs"; path = vpsfPkgs; }
      ];
    };
  };

  /*
  "old" = { config, pkgs, ... }: {
    boot.isContainer = true;

    deployment = {
      nixPath = [
        { prefix = "nixpkgs"; path = oldPkgs.path; }
      ];
    };
  };
  */
}
