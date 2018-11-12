{ pkgs }: with pkgs.lib;
let
  read = dir: mapAttrs' (name: type:
    let
      uuid = removeSuffix ".json" name;
    in
      nameValuePair uuid ((builtins.fromJSON (builtins.readFile (dir + "/${name}"))) // { inherit uuid; })
  )
    (filterAttrs (name: type: type == "regular" && hasSuffix ".json" name && name != "default.nix")
      (builtins.readDir dir));
in
  read ../hosts
