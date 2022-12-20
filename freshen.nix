{ buildGoModule, makeWrapper, nix }:
buildGoModule {
  name = "freshen";
  src = ./.;
  vendorHash = null;
  nativeBuildInputs = [ makeWrapper ];
  postInstall = ''
    wrapProgram $out/bin/freshen --prefix PATH : ${nix}/bin
  '';
}
