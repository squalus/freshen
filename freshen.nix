{ buildGoModule }:
buildGoModule {
  name = "freshen";
  src = ./.;
  vendorHash = null;
}
