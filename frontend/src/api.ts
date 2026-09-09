// Thin fetch wrappers over the endpoints documented in README.md ("walkmap
// serve"). All requests are same-origin (relative paths): in dev, vite's
// proxy forwards /api and /basemap.pmtiles to `just serve`; in prod, the Go
// server serves both the SPA and the API from one origin.

export interface Place {
  id: string;
  source: "osm" | "overture";
  name: string | null;
  category: string;
  confidence: number | null;
  lat: number;
  lon: number;
  walk_minutes?: number;
  walk_meters?: number;
  distance_m?: number;
}

export interface NearbyResponse {
  // A GeoJSON Polygon/MultiPolygon; typed loosely here since the SPA only
  // ever hands it straight to a maplibre GeoJSON source.
  isochrone: { type: string; coordinates: unknown };
  sorted_by: "walk_time" | "distance";
  places: Place[];
}

export interface CategoryCount {
  category: string;
  counts: Record<string, number>;
}

export interface RouteResponse {
  shape: [number, number][]; // [lat, lon] pairs, decoded server-side
  minutes: number;
  meters: number;
}

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message);
  }
}

type Params = Record<string, string | number | string[] | undefined>;

async function getJSON<T>(path: string, params: Params): Promise<T> {
  const url = new URL(path, window.location.origin);
  for (const [key, value] of Object.entries(params)) {
    if (value === undefined) continue;
    if (Array.isArray(value)) {
      for (const v of value) url.searchParams.append(key, v);
    } else {
      url.searchParams.set(key, String(value));
    }
  }
  const res = await fetch(url);
  if (!res.ok) {
    const body = await res.json().catch(() => ({}) as { error?: string });
    throw new ApiError(res.status, body.error ?? `${path} failed: ${res.status}`);
  }
  return res.json() as Promise<T>;
}

export function fetchCategories(): Promise<CategoryCount[]> {
  return getJSON("/api/categories", {});
}

export function fetchNearby(params: {
  lat: number;
  lon: number;
  category: string[];
  minutes: number;
  min_confidence: number;
}): Promise<NearbyResponse> {
  return getJSON("/api/nearby", params);
}

export function fetchSearch(params: {
  q: string;
  lat?: number;
  lon?: number;
  limit?: number;
}): Promise<{ places: Place[] }> {
  return getJSON("/api/search", params);
}

export function fetchRoute(params: {
  from_lat: number;
  from_lon: number;
  to_id: string;
}): Promise<RouteResponse> {
  return getJSON("/api/route", params);
}
