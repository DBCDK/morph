{
  projectRootFile = "flake.nix";
  programs = {
    nixpkgs-fmt.enable = true; # nix formatter
    statix.enable = true; # nix static analysis
    shellcheck.enable = true; # bash/shell
    taplo.enable = true; # toml
    yamlfmt.enable = true; # yaml
    gofmt.enable = true;
  };
  settings = {
    formatter = {
      nixpkgs-fmt.includes = [ "*.nix" "./data/*" ];
      statix.includes = [ "*.nix" "./data/*" ];
    };
  };
}
