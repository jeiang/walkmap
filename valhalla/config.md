# Running the Valhalla service

After `build-tiles.sh <pbf> <out-dir>` has produced `<out-dir>/current`:

```
valhalla_service <out-dir>/current/valhalla.json 1
```

The trailing `1` is the worker thread count. `valhalla_service` listens on
port **8002** by default (`mjolnir.service.listen`), which matches
`VALHALLA_URL`'s default of `http://127.0.0.1:8002` in the Go service.
Keep it bound to loopback (the default) — it has no auth of its own.
