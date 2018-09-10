set -eu

WD=$(mktemp -d)
GIT_REV=$(git rev-parse HEAD)

dep2nix -i Gopkg.lock -o "$WD/deps.nix"

cp nix-packaging/template/default.nix "$WD"
sed -e s/__VERSION__/${GIT_REV}/g "$WD/default.nix" -i

echo nix-shell -E "with import <nixpkgs> {}; callPackage $WD/default.nix {}"
SHA256=$(nix-shell -E "with import <nixpkgs> {}; callPackage $WD/default.nix {}" 2>&1 >/dev/null | grep "0000000000000000000000000000000000000000000000000000" | cut -d " " -f9 | tr -d "'")
sed -e "s/0000000000000000000000000000000000000000000000000000/${SHA256}/g" "$WD/default.nix" -i

echo "$WD"