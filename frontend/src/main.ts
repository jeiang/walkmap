import * as maplibregl from "maplibre-gl";
import "maplibre-gl/dist/maplibre-gl.css";
import { Protocol } from "pmtiles";
import { ApiError, fetchCategories, fetchNearby, fetchRoute, fetchSearch, type Place } from "./api";
import { CATEGORIES, categoryColor, categoryLabel } from "./taxonomy";

const BRIDGETOWN: [number, number] = [-59.6165, 13.0975]; // [lon, lat]

const protocol = new Protocol();
maplibregl.addProtocol("pmtiles", protocol.tile);

const map = new maplibregl.Map({
  container: "map",
  style: "/style/style.json",
  center: BRIDGETOWN,
  zoom: 14,
});
map.addControl(new maplibregl.NavigationControl({ showCompass: false }), "top-right");

const geolocate = new maplibregl.GeolocateControl({
  positionOptions: { enableHighAccuracy: true },
  trackUserLocation: true,
});
map.addControl(geolocate, "top-right");

// ---- state ----

let origin: { lat: number; lon: number } | null = null;
const selectedCategories = new Set<string>();
let minutes = 20;
let minConfidence = 0.5;
let searchQuery = "";
let nearbyPlaces: Place[] = [];
let requestSeq = 0;

const originMarker = new maplibregl.Marker({ color: "#2a6ef0" });

// ---- DOM ----

const searchInput = document.getElementById("search") as HTMLInputElement;
const categoriesEl = document.getElementById("categories") as HTMLDivElement;
const minutesEl = document.getElementById("minutes") as HTMLDivElement;
const confidenceInput = document.getElementById("confidence") as HTMLInputElement;
const confidenceValue = document.getElementById("confidence-value") as HTMLSpanElement;
const resultsEl = document.getElementById("results") as HTMLUListElement;
const toastEl = document.getElementById("toast") as HTMLDivElement;
const routeBanner = document.getElementById("route-banner") as HTMLDivElement;
const routeSummary = document.getElementById("route-summary") as HTMLSpanElement;
const routeClose = document.getElementById("route-close") as HTMLButtonElement;

for (const category of CATEGORIES) {
  const chip = document.createElement("button");
  chip.className = "chip";
  chip.dataset.category = category;
  chip.innerHTML = `<span class="dot" style="background:${categoryColor(category)}"></span>${categoryLabel(category)}`;
  chip.addEventListener("click", () => {
    if (selectedCategories.has(category)) {
      selectedCategories.delete(category);
      chip.classList.remove("selected");
    } else {
      selectedCategories.add(category);
      chip.classList.add("selected");
    }
    refreshNearby();
  });
  categoriesEl.appendChild(chip);
}

minutesEl.addEventListener("click", (e) => {
  const btn = (e.target as HTMLElement).closest("button[data-minutes]") as HTMLButtonElement | null;
  if (!btn) return;
  minutesEl.querySelectorAll("button").forEach((b) => b.classList.remove("active"));
  btn.classList.add("active");
  minutes = Number(btn.dataset.minutes);
  refreshNearby();
});

confidenceInput.addEventListener("input", () => {
  minConfidence = Number(confidenceInput.value);
  confidenceValue.textContent = minConfidence.toFixed(2);
  refreshNearby();
});

let searchDebounce: ReturnType<typeof setTimeout> | undefined;
searchInput.addEventListener("input", () => {
  searchQuery = searchInput.value.trim();
  clearTimeout(searchDebounce);
  searchDebounce = setTimeout(runSearch, 300);
});

routeClose.addEventListener("click", () => {
  routeBanner.hidden = true;
  clearRoute();
});

// ---- toast ----

let toastTimer: ReturnType<typeof setTimeout> | undefined;
function showToast(message: string) {
  toastEl.textContent = message;
  toastEl.hidden = false;
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => {
    toastEl.hidden = true;
  }, 4000);
}

function reportError(err: unknown, fallback: string) {
  if (err instanceof ApiError && err.status === 502) {
    showToast("Routing service is unavailable right now. Try again shortly.");
    return;
  }
  const message = err instanceof Error ? err.message : fallback;
  showToast(message);
}

// ---- origin ----

function setOrigin(lon: number, lat: number) {
  origin = { lat, lon };
  originMarker.setLngLat([lon, lat]).addTo(map);
  refreshNearby();
}

map.on("click", (e: maplibregl.MapMouseEvent) => {
  const features = map.queryRenderedFeatures(e.point, { layers: ["places-circle"] });
  if (features.length > 0) {
    const place = JSON.parse(features[0].properties?.place as string) as Place;
    void routeTo(place);
    return;
  }
  setOrigin(e.lngLat.lng, e.lngLat.lat);
});

geolocate.on("geolocate", (e: { coords: { longitude: number; latitude: number } }) => {
  setOrigin(e.coords.longitude, e.coords.latitude);
});

// ---- map layers (added once the vendored style has loaded) ----

function ensureSources() {
  if (!map.getSource("isochrone")) {
    map.addSource("isochrone", { type: "geojson", data: emptyFeatureCollection() });
    map.addLayer({
      id: "isochrone-fill",
      type: "fill",
      source: "isochrone",
      paint: { "fill-color": "#2a6ef0", "fill-opacity": 0.12 },
    });
    map.addLayer({
      id: "isochrone-line",
      type: "line",
      source: "isochrone",
      paint: { "line-color": "#2a6ef0", "line-width": 2 },
    });
  }
  if (!map.getSource("places")) {
    map.addSource("places", { type: "geojson", data: emptyFeatureCollection() });
    // The match/case expressions below are typed as tuples by maplibre's
    // style spec; a spread of a dynamically-built stop list defeats that
    // tuple inference, so build them as plain arrays and cast once.
    const colorByCategory = ["match", ["get", "category"], ...categoryMatchStops(), "#666666"];
    map.addLayer({
      id: "places-circle",
      type: "circle",
      source: "places",
      paint: {
        "circle-radius": 7,
        "circle-color": colorByCategory as maplibregl.DataDrivenPropertyValueSpecification<string>,
        "circle-opacity": ["case", ["==", ["get", "source"], "osm"], 1, 0],
        "circle-stroke-color": colorByCategory as maplibregl.DataDrivenPropertyValueSpecification<string>,
        "circle-stroke-width": ["case", ["==", ["get", "source"], "osm"], 1, 2],
        "circle-stroke-opacity": 1,
      },
    });
  }
  if (!map.getSource("route")) {
    map.addSource("route", { type: "geojson", data: emptyFeatureCollection() });
    map.addLayer({
      id: "route-line",
      type: "line",
      source: "route",
      paint: { "line-color": "#2a6ef0", "line-width": 4 },
    });
  }
}

function categoryMatchStops(): string[] {
  const stops: string[] = [];
  for (const c of CATEGORIES) {
    stops.push(c, categoryColor(c));
  }
  return stops;
}

function emptyFeatureCollection(): GeoJSON.FeatureCollection {
  return { type: "FeatureCollection", features: [] };
}

if (map.isStyleLoaded()) ensureSources();
map.on("load", ensureSources);

// ---- nearby ----

async function refreshNearby() {
  if (!origin || selectedCategories.size === 0) {
    nearbyPlaces = [];
    setIsochrone(null);
    setPlaces([]);
    if (!searchQuery) renderResults([], undefined);
    return;
  }
  const seq = ++requestSeq;
  try {
    const res = await fetchNearby({
      lat: origin.lat,
      lon: origin.lon,
      category: [...selectedCategories],
      minutes,
      min_confidence: minConfidence,
    });
    if (seq !== requestSeq) return; // a newer request has since landed
    nearbyPlaces = res.places;
    setIsochrone(res.isochrone);
    setPlaces(res.places);
    if (!searchQuery) renderResults(res.places, res.sorted_by);
  } catch (err) {
    if (seq !== requestSeq) return;
    reportError(err, "Could not load nearby places.");
  }
}

function setIsochrone(geometry: { type: string; coordinates: unknown } | null) {
  const source = map.getSource("isochrone") as maplibregl.GeoJSONSource | undefined;
  if (!source) return;
  source.setData(
    geometry
      ? { type: "Feature", geometry: geometry as unknown as GeoJSON.Geometry, properties: {} }
      : emptyFeatureCollection(),
  );
}

function setPlaces(places: Place[]) {
  const source = map.getSource("places") as maplibregl.GeoJSONSource | undefined;
  if (!source) return;
  source.setData({
    type: "FeatureCollection",
    features: places.map((p) => ({
      type: "Feature" as const,
      geometry: { type: "Point" as const, coordinates: [p.lon, p.lat] },
      properties: { category: p.category, source: p.source, place: JSON.stringify(p) },
    })),
  });
}

// ---- search ----

async function runSearch() {
  if (!searchQuery) {
    renderResults(nearbyPlaces, nearbyPlaces.length ? "walk_time" : undefined);
    return;
  }
  const seq = ++requestSeq;
  try {
    const res = await fetchSearch({
      q: searchQuery,
      lat: origin?.lat,
      lon: origin?.lon,
      limit: 20,
    });
    if (seq !== requestSeq) return;
    renderResults(res.places, undefined);
  } catch (err) {
    if (seq !== requestSeq) return;
    reportError(err, "Search failed.");
  }
}

// ---- results list ----

function renderResults(places: Place[], sortedBy: "walk_time" | "distance" | undefined) {
  resultsEl.innerHTML = "";
  for (const place of places) {
    const li = document.createElement("li");
    li.className = "result";

    const marker = document.createElement("span");
    marker.className = `marker ${place.source}`;
    marker.style.color = categoryColor(place.category);
    if (place.source === "osm") marker.style.background = categoryColor(place.category);
    li.appendChild(marker);

    const info = document.createElement("div");
    info.className = "info";
    const name = document.createElement("div");
    name.className = "name";
    name.textContent = place.name ?? `Unnamed ${categoryLabel(place.category)}`;
    const meta = document.createElement("div");
    meta.className = "meta";
    meta.textContent = `${categoryLabel(place.category)} · ${metaText(place, sortedBy)}`;
    info.append(name, meta);
    li.appendChild(info);

    if (place.source === "osm") {
      const badge = document.createElement("span");
      badge.className = "badge";
      badge.textContent = "verified";
      li.appendChild(badge);
    } else if (place.confidence != null) {
      const badge = document.createElement("span");
      badge.className = "badge";
      badge.textContent = `${Math.round(place.confidence * 100)}%`;
      li.appendChild(badge);
    }

    li.addEventListener("click", () => void routeTo(place));
    resultsEl.appendChild(li);
  }
}

function metaText(place: Place, sortedBy: "walk_time" | "distance" | undefined): string {
  if (sortedBy === "walk_time" && place.walk_minutes != null) {
    return `${Math.round(place.walk_minutes)} min walk`;
  }
  if (place.distance_m != null) {
    return `${Math.round(place.distance_m)} m away`;
  }
  return "";
}

// ---- routing ----

function clearRoute() {
  const source = map.getSource("route") as maplibregl.GeoJSONSource | undefined;
  source?.setData(emptyFeatureCollection());
}

async function routeTo(place: Place) {
  if (!origin) {
    showToast("Tap the map or use the geolocate button to set your starting point first.");
    return;
  }
  try {
    const res = await fetchRoute({ from_lat: origin.lat, from_lon: origin.lon, to_id: place.id });
    const source = map.getSource("route") as maplibregl.GeoJSONSource | undefined;
    source?.setData({
      type: "Feature",
      geometry: {
        type: "LineString",
        coordinates: res.shape.map(([lat, lon]) => [lon, lat]),
      },
      properties: {},
    });
    routeSummary.textContent = `${Math.round(res.minutes)} min · ${Math.round(res.meters)} m to ${place.name ?? categoryLabel(place.category)}`;
    routeBanner.hidden = false;
  } catch (err) {
    reportError(err, "Could not get a route to that place.");
  }
}

// ---- init ----

// Chips are built eagerly from the full taxonomy (above) so every category
// shows even with zero rows today; /api/categories only adds a live count
// per chip.
fetchCategories()
  .then((rows) => {
    const totals = new Map(rows.map((r) => [r.category, Object.values(r.counts).reduce((a, b) => a + b, 0)]));
    categoriesEl.querySelectorAll<HTMLButtonElement>(".chip").forEach((chip) => {
      const count = totals.get(chip.dataset.category ?? "") ?? 0;
      const span = document.createElement("span");
      span.className = "count";
      span.textContent = ` (${count})`;
      chip.appendChild(span);
    });
  })
  .catch((err) => reportError(err, "Could not load categories."));
