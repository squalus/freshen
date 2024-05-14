{ lib, buildGoModule, makeWrapper, nix }:
buildGoModule {
  name = "freshen";
  src = ./.;
  vendorHash = "sha256-tWFDM9loLCNfd9weDJZ3JabhISDsDcVaTKeshBi52VI=";
  nativeBuildInputs = [ makeWrapper ];
  postInstall = ''
    wrapProgram $out/bin/freshen --prefix PATH : ${nix}/bin
  '';
}
