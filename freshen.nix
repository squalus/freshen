{ lib, buildGoModule, makeWrapper, nix }:
buildGoModule {
  name = "freshen";
  src = ./.;
  vendorHash = "sha256-A5tJz/2jdAFxzaGIuAvZHW5x1yXYTZjhd7l/OADiKSM=";
  nativeBuildInputs = [ makeWrapper ];
  postInstall = ''
    wrapProgram $out/bin/freshen --prefix PATH : ${nix}/bin
  '';
}
