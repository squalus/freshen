{ lib, buildGoModule, makeWrapper, nix }:
buildGoModule {
  name = "freshen";
  src = ./.;
  vendorHash = "sha256-Z+m489SRfolzhRwksFl0ZA5E5RLW5hP+nQoAPRKlr94=";
  nativeBuildInputs = [ makeWrapper ];
  postInstall = ''
    wrapProgram $out/bin/freshen --prefix PATH : ${nix}/bin
  '';
}
