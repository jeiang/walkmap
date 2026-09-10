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

    nixosModules.default = import ./nix/module.nix {inherit self;};

    checks.x86_64-linux = let
      pkgs = nixpkgs.legacyPackages.x86_64-linux;

      # Cross eval only (docs: nix eval .#checks.x86_64-linux.module-eval.drvPath
      # works from aarch64-darwin) -- a NixOS system for x86_64-linux can't be
      # *built* from Darwin, but evaluating one to a derivation is a plain
      # cross-system eval, no build required.
      moduleEval = nixpkgs.lib.nixosSystem {
        system = "x86_64-linux";
        modules = [
          self.nixosModules.default
          {
            services.walkmap.enable = true;
            # nixosSystem needs a filesystem for eval-time checks even
            # though nothing is built.
            fileSystems."/" = {
              device = "/dev/null";
              fsType = "ext4";
            };
            boot.loader.grub.enable = false;
            system.stateVersion = "24.11";
          }
        ];
      };
    in {
      module-eval = moduleEval.config.system.build.toplevel;

      vm = pkgs.testers.nixosTest {
        name = "walkmap-module";
        nodes.machine = {pkgs, ...}: {
          imports = [self.nixosModules.default];
          services.walkmap = {
            enable = true;
            listenAddress = "0.0.0.0";
          };
        };
        testScript = ''
          machine.wait_for_unit("walkmap-db-init.service")

          # Seed fixture data directly with `walkmap import`, bypassing the
          # network-fetching osm.sh/overture.sh (no network in the VM).
          machine.succeed(
              "install -d -o walkmap -g walkmap /var/lib/walkmap/import"
          )
          machine.copy_from_host(
              "${./nix/fixtures/places.json}", "/var/lib/walkmap/import/places.json"
          )
          machine.copy_from_host(
              "${./nix/fixtures/overture.csv}", "/var/lib/walkmap/import/overture.csv"
          )
          machine.succeed(
              "runuser -u walkmap -- env DATABASE_URL='postgres:///walkmap?host=/run/postgresql' "
              + "${self.packages.x86_64-linux.walkmap}/bin/walkmap import osm /var/lib/walkmap/import/places.json"
          )
          machine.succeed(
              "runuser -u walkmap -- env DATABASE_URL='postgres:///walkmap?host=/run/postgresql' "
              + "${self.packages.x86_64-linux.walkmap}/bin/walkmap import overture /var/lib/walkmap/import/overture.csv"
          )

          machine.wait_for_unit("walkmap.service")
          machine.wait_for_open_port(8867)

          # Valhalla has no tiles in this test (no network to build them,
          # see docs on the skipped tile build below) so /healthz must
          # report 503 until an operator builds them.
          machine.succeed("curl -sf -o /dev/null -w '%{http_code}' http://localhost:8867/healthz | grep -q 503")

          machine.succeed("curl -sf http://localhost:8867/api/categories | grep -q convenience")
          machine.succeed("curl -sf 'http://localhost:8867/api/search?q=mini' | grep -q 'Mini Mart'")
        '';
      };
    };
  };
}
