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
    packages = forAll (pkgs: let
      walkmap = pkgs.buildGoModule {
        pname = "walkmap";
        version = "0.1.0";
        src = ./.;
        vendorHash = "sha256-6O546ro3KRsOWTkLxPaOa9RZ5HQbxUy3o3OiJ7+vYPk=";
      };

      # prebuild runs scripts/copy-maplibre-worker.mjs (docs/WALKMAP.md piece
      # 3), so the default `npm run build` target -- not a hand-picked
      # `vite build` -- is what buildNpmPackage must run.
      frontend = pkgs.buildNpmPackage {
        pname = "walkmap-frontend";
        version = "0.1.0";
        src = ./frontend;
        npmDepsHash = "sha256-lHnGttJGaDJGDeqCzRYTrTm5JO10La+jveq8J2MHjVU=";
        installPhase = ''
          runHook preInstall
          cp -r dist $out
          runHook postInstall
        '';
      };

      # Thin runtimeInputs wrappers around the existing shell scripts, one
      # store path each, for the NixOS module's systemd units to call.
      # Linux-only: nixpkgs' valhalla and tilemaker have no darwin build,
      # so these can't be defined (or built) there either.
      linuxOnly = pkgs.lib.optionalAttrs (pkgs.stdenv.hostPlatform.isLinux) {
        inherit (pkgs) valhalla;

        import-osm = pkgs.writeShellApplication {
          name = "walkmap-import-osm";
          runtimeInputs = [pkgs.curl pkgs.osmium-tool];
          text = ''
            exec ${./import/osm.sh} "$@"
          '';
        };

        import-overture = pkgs.writeShellApplication {
          name = "walkmap-import-overture";
          runtimeInputs = [pkgs.duckdb];
          text = ''
            exec ${./import/overture.sh} "$@"
          '';
        };

        valhalla-build-tiles = pkgs.writeShellApplication {
          name = "walkmap-valhalla-build-tiles";
          runtimeInputs = [pkgs.valhalla pkgs.jq];
          text = ''
            exec ${./valhalla/build-tiles.sh} "$@"
          '';
        };

        build-basemap = pkgs.writeShellApplication {
          name = "walkmap-build-basemap";
          runtimeInputs = [pkgs.tilemaker pkgs.curl pkgs.unzip];
          text = ''
            exec ${./basemap/build-pmtiles.sh} "$@"
          '';
        };
      };
    in
      {
        inherit walkmap frontend;
        default = walkmap;
      }
      // linuxOnly);

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
          pkgs.just
          pkgs.tilemaker
          pkgs.pmtiles
          pkgs.nodejs_24
        ];
      };
    });
  };
}
