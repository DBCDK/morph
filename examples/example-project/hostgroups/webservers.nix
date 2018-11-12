let
  pkgs = import ../common/nixpkgs.nix { version = "18.09"; };
  hosts = (import ../hosts){ inherit pkgs; };

  nginx = { config, pkgs, ... }:
  {
    services.nginx.enable = true;
  };

  createWebServer = { uuid, hosts}: { config, pkgs, ... }:
  let
    host = hosts."${uuid}";
  in
  {
    imports = [
      ../common
      nginx
    ];

    networking = {
      inherit (host) hostName;
      useDHCP = true;
    };
  };

in
{
  network =  {
    inherit pkgs;
    description = "Kea dhcp server";
  };

  "web01.example.com" = createWebServer {
    uuid = "00000000-0000-0000-0000-000000000001";
    inherit hosts;
  };

  "web02.example.com" = createWebServer {
    uuid = "00000000-0000-0000-0000-000000000002";
    inherit hosts;
  };
}
