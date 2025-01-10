{
  nixpkgs ? import ./nixpkgs.nix,
  pkgs ? import nixpkgs { },
  version ? "dev",
}:

pkgs.buildGoModule rec {
  name = "morph-unstable-${version}";
  inherit version;

  src = pkgs.nix-gitignore.gitignoreSource [ ] ./.;

  ldflags = [
    "-X main.version=${version}"
    "-X main.assetRoot=${placeholder "lib"}"
  ];

  nativeBuildInputs = [ pkgs.installShellFiles ];

  vendorHash = "sha256-Mi0SdvmYao6rLt8+bFcUv2AjHkJTLP85zGka1/cCPzQ=";

  postInstall = ''
    mkdir -p $lib
    cp -v ./data/*.nix $lib
    installShellCompletion --cmd morph \
      --bash <($out/bin/morph --completion-script-bash) \
      --zsh <($out/bin/morph --completion-script-zsh)
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
}
