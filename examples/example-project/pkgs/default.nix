# Nix overlay. You can add your own packages here.
self: super:
with super;
{
  morph = with builtins; with super;
  let
      pin = fromJSON (readFile ../morph.json);
      checkout = fetchgit { inherit (pin) url rev sha256; };
  in
    callPackage "${checkout}/nix-packaging/default.nix" { version = pin.rev; };
}
