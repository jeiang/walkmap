{
  description = "walkmap: category search by walking time for Barbados";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs = {
    self,
    nixpkgs,
  }: let
    systems = ["x86_64-linux" "aarch64-darwin"];
    forAll = f: nixpkgs.lib.genAttrs systems (system: f nixpkgs.legacyPackages.${system});
  in {
    packages = forAll (pkgs: rec {
      walkmap = pkgs.buildGoModule {
        pname = "walkmap";
        version = "0.1.0";
        src = ./.;
        vendorHash = "sha256-i5x5ILFLqSnd0FDEC+wJBdj/f95K+aB9fIEBPxd2ZiA=";
      };
      default = walkmap;
    });

    devShells = forAll (pkgs: {
      default = pkgs.mkShell {
        # withPackages so postgis lands on the same postgres's pkglibdir;
        # two separate store paths would leave `create extension postgis`
        # unable to find the extension.
        packages = [
          pkgs.go
          (pkgs.postgresql_17.withPackages (ps: [ps.postgis]))
          pkgs.duckdb
          pkgs.osmium-tool
          pkgs.curl
          pkgs.jq
        ];
      };
    });
  };
}
