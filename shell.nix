{ nixpkgs ? import ./nixpkgs.nix
, pkgs ? import nixpkgs {}
}:

let
  morph = pkgs.callPackage ./default.nix {};
  gen-assets = pkgs.writeShellScriptBin "gen-assets" ''
    ${pkgs.go-bindata}/bin/go-bindata -pkg assets -o assets/assets.go data/
  '';
in

pkgs.mkShell {
  buildInputs = [
    gen-assets
  ];

  inputsFrom = [
    morph
  ];
}
