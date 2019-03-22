{ lib }:

with builtins; with lib;
let
  blacklistedDirs = [ "nix-packaging" "vendor" "^\\..+$" ];
  whitelistedFiles = [ "^.+\\.nix$" "^.+\\.go$" "^.+\\.mod$" "^.+\\.sum$"];
  filterList = file: list: elem true (map (pattern: isList (match pattern file)) list);
  srcFilter = path: type: (
    if type == "regular" then filterList (baseNameOf path) whitelistedFiles
    else if type == "directory" then !filterList (baseNameOf path) blacklistedDirs
    else false);

in filterSource srcFilter ./..
