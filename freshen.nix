{ lib, buildGoModule, makeWrapper, nix }:
buildGoModule {
  name = "freshen";
  src = ./.;
  vendorHash = "sha256-WLgSNAo7TOM/cBNFmybmChxAEssJOGjEr2mZkSoZiYQ=";
  nativeBuildInputs = [ makeWrapper ];
  postInstall = ''
    wrapProgram $out/bin/freshen --prefix PATH : ${nix}/bin
  '';
}
