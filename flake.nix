{
  description = "Freshen";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "flake:flake-utils";
  };

  outputs = inputs@{ self, nixpkgs, flake-utils, ... }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages."${system}";
      in
      with pkgs;
      {
        packages.default = callPackage ./freshen.nix {};
        apps.default = {
          type = "app";
          program = "${self.packages.${system}.default}/bin/freshen";
        };
        devShells.default = pkgs.mkShell {
          inputsFrom = [
            self.packages.${system}.default
          ];
          nativeBuildInputs = [
            pkgs.golangci-lint
          ];
        };
      });
}
