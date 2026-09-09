// Mirrors internal/taxonomy/taxonomy.go's app category set (docs/WALKMAP.md,
// "App taxonomy and mapping"), so every category shows as a chip even when
// /api/categories has zero rows for it today.
export const CATEGORIES = [
  "convenience",
  "supermarket",
  "restaurant",
  "fast_food",
  "cafe",
  "bar",
  "pharmacy",
  "fuel",
  "bakery",
  "hardware",
  "bank_atm",
] as const;

export type Category = (typeof CATEGORIES)[number];

export const CATEGORY_LABELS: Record<string, string> = {
  convenience: "Convenience",
  supermarket: "Supermarket",
  restaurant: "Restaurant",
  fast_food: "Fast Food",
  cafe: "Cafe",
  bar: "Bar",
  pharmacy: "Pharmacy",
  fuel: "Fuel",
  bakery: "Bakery",
  hardware: "Hardware",
  bank_atm: "Bank/ATM",
};

// One distinct color per category, used for both chips and map markers.
export const CATEGORY_COLORS: Record<string, string> = {
  convenience: "#e6194b",
  supermarket: "#3cb44b",
  restaurant: "#f58231",
  fast_food: "#ffe119",
  cafe: "#9a6324",
  bar: "#911eb4",
  pharmacy: "#42d4f4",
  fuel: "#4363d8",
  bakery: "#f032e6",
  hardware: "#808000",
  bank_atm: "#469990",
};

export function categoryLabel(category: string): string {
  return CATEGORY_LABELS[category] ?? category;
}

export function categoryColor(category: string): string {
  return CATEGORY_COLORS[category] ?? "#666666";
}
