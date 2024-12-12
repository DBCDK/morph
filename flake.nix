{
  description = "A very basic flake";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-24.05";

    flake-utils = {
      url = "github:numtide/flake-utils";
    };

    pre-commit-hooks = {
      url = "github:cachix/pre-commit-hooks.nix";
      inputs.nixpkgs.follows = "nixpkgs";
      inputs.flake-utils.follows = "flake-utils";
    };

    treefmt-nix = {
      url = "github:numtide/treefmt-nix";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs =
    {
      self,
      pre-commit-hooks,
      nixpkgs,
      flake-utils,
      treefmt-nix,
    }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        # Current Version of Morph
        # TODO: this sucks...
        version = "dev";

        pkgs = (import nixpkgs) { inherit system; };

        # Eval the treefmt modules from ./treefmt.nix
        treefmtEval = treefmt-nix.lib.evalModule pkgs ./treefmt.nix;
      in
      rec {
        # for `nix fmt`
        formatter = treefmtEval.config.build.wrapper;

        # for `nix flake check`
        checks = {
          vm_integration_tests = pkgs.callPackage ./nixos/tests/integration_tests.nix { inherit packages; };
          formatting = treefmtEval.config.build.check self;
          build = self.packages.${system}.morph;
          pre-commit-check =
            let
              # some treefmt formatters are not supported in pre-commit-hooks we
              # filter them out for now.
              toFilter = [
                "yamlfmt"
                "nixfmt"
              ];
              filterFn = n: _v: (!builtins.elem n toFilter);
              treefmtFormatters = pkgs.lib.mapAttrs (_n: v: { inherit (v) enable; }) (
                pkgs.lib.filterAttrs filterFn (import ./treefmt.nix).programs
              );
            in
            pre-commit-hooks.lib.${system}.run {
              src = ./.;
              hooks = treefmtFormatters // {
                nixfmt-rfc-style.enable = true;
              };
            };
        };

        # Acessible through 'nix develop' or 'nix-shell' (legacy)
        devShells.default = pkgs.mkShell {
          inherit (self.checks.${system}.pre-commit-check) shellHook;
          inputsFrom = [ self.packages.${system}.morph ];
        };

        packages = rec {
          default = morph;
          morph = pkgs.buildGoModule rec {
            name = "morph-unstable-${version}";
            inherit version;

            src = pkgs.nix-gitignore.gitignoreSource [ ] ./.;

            ldflags = [
              "-X main.version=${version}"
              "-X main.assetRoot=${placeholder "lib"}"
            ];

            vendorHash = "sha256-Mi0SdvmYao6rLt8+bFcUv2AjHkJTLP85zGka1/cCPzQ=";

            postInstall = ''
              mkdir -p $lib
              cp -v ./data/*.nix $lib
            '';

            outputs = [
              "out"
              "lib"
            ];

            meta = {
              homepage = "https://github.com/DBCDK/morph";
              description = "Morph is a NixOS host manager written in Golang.";
              mainProgram = "morph";
            };
          };
        };
      }
    );
}
