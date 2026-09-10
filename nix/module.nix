# NixOS module for the walkmap service (docs/WALKMAP.md, piece 4).
#
# Takes `self` so its systemd units can reach this flake's own packages
# (the Go binary, the built frontend, and the Linux-only script wrappers)
# for the running system, the same way color-hunt.nix's consumer wires
# `inputs.color-hunt.nixosModules`.
{self}: {
  config,
  lib,
  pkgs,
  ...
}: let
  cfg = config.services.walkmap;
  system = pkgs.stdenv.hostPlatform.system;
  flakePkgs = self.packages.${system};

  # Peer-authed over the postgres unix socket as cfg.user; ensureUsers below
  # creates a matching role.
  databaseUrl = "postgres:///walkmap?host=/run/postgresql";

  installDataDirs = dirs: "+${pkgs.coreutils}/bin/install -d -o ${cfg.user} -g ${cfg.group} " + lib.concatMapStringsSep " " (d: "${cfg.dataDir}/${d}") dirs;

  # Hardening shared by every walkmap-owned unit; each caller adds its own
  # ReadOnlyPaths/ReadWritePaths on top (color-hunt.nix/wger's rigor).
  commonHardening = {
    NoNewPrivileges = true;
    PrivateTmp = true;
    ProtectSystem = "strict";
    ProtectHome = true;
    ProtectClock = true;
    ProtectProc = "invisible";
    ProtectKernelLogs = true;
    ProtectKernelModules = true;
    ProtectKernelTunables = true;
    ProtectControlGroups = true;
    ProtectHostname = true;
    RestrictSUIDSGID = true;
    RestrictRealtime = true;
    RestrictNamespaces = true;
    LockPersonality = true;
  };
in {
  options.services.walkmap = {
    enable = lib.mkEnableOption "walkmap: category search by walking time";

    port = lib.mkOption {
      type = lib.types.port;
      default = 8867;
      description = "Port walkmap's API/SPA listens on.";
    };

    listenAddress = lib.mkOption {
      type = lib.types.str;
      default = "127.0.0.1";
      description = ''
        Address walkmap binds. No firewall option is provided here: set
        this to the mesh interface or 0.0.0.0 and scope the consumer's own
        firewall, the same way as wger.
      '';
    };

    dataDir = lib.mkOption {
      type = lib.types.path;
      default = "/var/lib/walkmap";
      description = "Import output, Valhalla tiles, and the built basemap. Persist this.";
    };

    user = lib.mkOption {
      type = lib.types.str;
      default = "walkmap";
    };

    group = lib.mkOption {
      type = lib.types.str;
      default = "walkmap";
    };

    package = lib.mkOption {
      type = lib.types.package;
      default = flakePkgs.default;
      description = "The walkmap Go binary.";
    };

    frontendPackage = lib.mkOption {
      type = lib.types.package;
      default = flakePkgs.frontend;
      description = "The built SPA, served at / (WALKMAP_STATIC_DIR).";
    };

    valhallaPackage = lib.mkOption {
      type = lib.types.package;
      default = pkgs.valhalla;
      description = "Package providing valhalla_service, for walkmap-valhalla.service.";
    };

    valhallaPort = lib.mkOption {
      type = lib.types.port;
      default = 8002;
      description = "Loopback-only port for the Valhalla routing service.";
    };

    overtureRelease = lib.mkOption {
      type = lib.types.str;
      default = "2026-08-19.0";
      description = "Overture Maps release partition to pull Places from.";
    };

    overpassUrl = lib.mkOption {
      type = lib.types.str;
      # import/osm.sh's own default (OVERPASS_URL); set here rather than
      # left unset so the timer-triggered unit doesn't depend on the
      # script's fallback.
      default = "https://overpass-api.de/api/interpreter";
      description = "Overpass instance for the nightly OSM refresh.";
    };

    osmRefresh = lib.mkOption {
      type = lib.types.str;
      default = "*-*-* 03:30:00";
      description = "systemd OnCalendar for walkmap-import-osm.timer.";
    };

    overtureRefresh = lib.mkOption {
      type = lib.types.str;
      default = "monthly";
      description = "systemd OnCalendar for walkmap-import-overture.timer.";
    };
  };

  config = lib.mkIf cfg.enable {
    users.groups.${cfg.group} = {};
    users.users.${cfg.user} = {
      isSystemUser = true;
      group = cfg.group;
    };

    services.postgresql = {
      enable = true;
      ensureDatabases = ["walkmap"];
      ensureUsers = [
        {
          name = "walkmap";
          ensureDBOwnership = true;
        }
      ];
      extensions = ps: [ps.postgis];
      # No `package` or `settings` here: this must merge cleanly with a
      # host that already runs postgresql for another service (e.g.
      # artemis's wger, which sets settings.wal_level = "logical").
    };

    # CREATE EXTENSION postgis needs superuser (it isn't marked trusted);
    # `walkmap migrate`'s own `create extension if not exists` runs as the
    # unprivileged walkmap role and is a no-op once this has run.
    systemd.services.walkmap-db-init = {
      description = "walkmap: create postgis/pg_trgm extensions";
      after = ["postgresql.service"];
      requires = ["postgresql.service"];
      serviceConfig = {
        Type = "oneshot";
        RemainAfterExit = true;
        User = "postgres";
      };
      path = [config.services.postgresql.package];
      script = ''
        psql -v ON_ERROR_STOP=1 -d walkmap -c "create extension if not exists postgis; create extension if not exists pg_trgm;"
      '';
    };

    systemd.services.walkmap = {
      description = "walkmap API server";
      after = ["postgresql.service" "network-online.target" "walkmap-db-init.service"];
      requires = ["walkmap-db-init.service"];
      # Wants, not Requires: the app must come up and serve /healthz 503
      # before Valhalla has tiles to serve (first-run sequence).
      wants = ["walkmap-valhalla.service" "network-online.target"];
      wantedBy = ["multi-user.target"];
      environment = {
        WALKMAP_LISTEN = "${cfg.listenAddress}:${toString cfg.port}";
        DATABASE_URL = databaseUrl;
        VALHALLA_URL = "http://127.0.0.1:${toString cfg.valhallaPort}";
        WALKMAP_STATIC_DIR = "${cfg.frontendPackage}";
        WALKMAP_BASEMAP = "${cfg.dataDir}/basemap/basemap.pmtiles";
      };
      serviceConfig =
        commonHardening
        // {
          ExecStartPre = [
            (installDataDirs ["basemap"])
            "${cfg.package}/bin/walkmap migrate"
          ];
          ExecStart = "${cfg.package}/bin/walkmap serve";
          User = cfg.user;
          Group = cfg.group;
          Restart = "on-failure";
          RestartSec = 5;
          # Only reads basemap.pmtiles from dataDir; never writes there.
          ReadOnlyPaths = ["-${cfg.dataDir}"];
          PrivateDevices = true;
          CapabilityBoundingSet = "";
          MemoryDenyWriteExecute = true;
          SystemCallFilter = ["@system-service" "~@privileged"];
        };
    };

    systemd.services.walkmap-valhalla = {
      description = "walkmap: Valhalla pedestrian routing service";
      after = ["network.target"];
      serviceConfig =
        commonHardening
        // {
          ExecStart = "${cfg.valhallaPackage}/bin/valhalla_service ${cfg.dataDir}/valhalla/current/valhalla.json 1";
          User = cfg.user;
          Group = cfg.group;
          # No tiles until the first walkmap-import-osm run; don't
          # crash-loop waiting for them.
          ConditionPathExists = "${cfg.dataDir}/valhalla/current/valhalla.json";
          Restart = "on-failure";
          RestartSec = 5;
          ReadOnlyPaths = ["-${cfg.dataDir}/valhalla"];
          PrivateDevices = true;
          CapabilityBoundingSet = "";
        };
    };

    # Runnable by hand (`systemctl start walkmap-import-osm`) as well as by
    # its timer. Each ExecStart= is its own oneshot step -- systemd aborts
    # the unit at the first one that fails, so this doesn't need `&&`
    # chaining or its own `set -e`.
    systemd.services.walkmap-import-osm = {
      description = "walkmap: nightly OSM places + road network import";
      after = ["walkmap-db-init.service" "network-online.target"];
      requires = ["walkmap-db-init.service"];
      wants = ["network-online.target"];
      environment = {
        DATABASE_URL = databaseUrl;
        OVERPASS_URL = cfg.overpassUrl;
        TILEMAKER_CACHE = "${cfg.dataDir}/basemap/cache";
      };
      serviceConfig =
        commonHardening
        // {
          Type = "oneshot";
          User = cfg.user;
          Group = cfg.group;
          ReadWritePaths = ["-${cfg.dataDir}"];
          ExecStartPre = installDataDirs ["import" "valhalla" "basemap/cache"];
          ExecStart = [
            "${flakePkgs.import-osm}/bin/walkmap-import-osm ${cfg.dataDir}/import"
            "${cfg.package}/bin/walkmap import osm ${cfg.dataDir}/import/places.json"
            "${cfg.package}/bin/walkmap import dedupe"
            "${flakePkgs.valhalla-build-tiles}/bin/walkmap-valhalla-build-tiles ${cfg.dataDir}/import/barbados.osm.pbf ${cfg.dataDir}/valhalla"
            "${flakePkgs.build-basemap}/bin/walkmap-build-basemap ${cfg.dataDir}/import/barbados.osm.pbf ${cfg.dataDir}/basemap"
          ];
          # try-restart, not restart: a Valhalla that was never up (no
          # prior tiles) has nothing to restart, and this shouldn't start
          # it outside of walkmap.service's own `wants` ordering. The `+`
          # runs it as root, bypassing polkit, same as color-hunt.nix's
          # ExecStartPre pattern.
          ExecStartPost = "+${pkgs.systemd}/bin/systemctl try-restart walkmap-valhalla.service";
        };
    };

    systemd.timers.walkmap-import-osm = {
      wantedBy = ["timers.target"];
      timerConfig = {
        OnCalendar = cfg.osmRefresh;
        Persistent = true;
      };
    };

    systemd.services.walkmap-import-overture = {
      description = "walkmap: monthly Overture Places import";
      after = ["walkmap-db-init.service" "network-online.target"];
      requires = ["walkmap-db-init.service"];
      wants = ["network-online.target"];
      environment.DATABASE_URL = databaseUrl;
      serviceConfig =
        commonHardening
        // {
          Type = "oneshot";
          User = cfg.user;
          Group = cfg.group;
          ReadWritePaths = ["-${cfg.dataDir}"];
          ExecStartPre = installDataDirs ["import"];
          ExecStart = [
            "${flakePkgs.import-overture}/bin/walkmap-import-overture ${cfg.dataDir}/import ${cfg.overtureRelease}"
            "${cfg.package}/bin/walkmap import overture ${cfg.dataDir}/import/overture.csv"
            "${cfg.package}/bin/walkmap import dedupe"
          ];
        };
    };

    systemd.timers.walkmap-import-overture = {
      wantedBy = ["timers.target"];
      timerConfig = {
        OnCalendar = cfg.overtureRefresh;
        Persistent = true;
      };
    };
  };
}
