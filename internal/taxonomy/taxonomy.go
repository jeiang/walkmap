// Package taxonomy maps OSM tags and Overture Places categories onto the
// app's fixed category set (docs/WALKMAP.md, "App taxonomy and mapping").
package taxonomy

import "strings"

// osmMap keys are "key=value" OSM tags that map directly to an app category.
var osmMap = map[string]string{
	"shop=convenience":   "convenience",
	"shop=kiosk":         "convenience",
	"shop=supermarket":   "supermarket",
	"amenity=restaurant": "restaurant",
	"amenity=fast_food":  "fast_food",
	"amenity=cafe":       "cafe",
	"amenity=bar":        "bar",
	"amenity=pub":        "bar",
	"amenity=pharmacy":   "pharmacy",
	"amenity=fuel":       "fuel",
	"shop=bakery":        "bakery",
	"shop=hardware":      "hardware",
	"shop=doityourself":  "hardware",
	"amenity=bank":       "bank_atm",
	"amenity=atm":        "bank_atm",
}

// OSMCategory returns the app category for a set of OSM tags, checking
// shop= then amenity= (the only two keys the taxonomy uses). ok is false
// when nothing in tags maps.
func OSMCategory(tags map[string]string) (category string, ok bool) {
	for _, key := range []string{"shop", "amenity"} {
		if v, present := tags[key]; present {
			if cat, mapped := osmMap[key+"="+v]; mapped {
				return cat, true
			}
		}
	}
	return "", false
}

// overtureMap keys are Overture categories.primary values.
var overtureMap = map[string]string{
	"convenience_store":    "convenience",
	"grocery_store":        "supermarket",
	"supermarket":          "supermarket",
	"restaurant":           "restaurant",
	"fast_food_restaurant": "fast_food",
	"cafe":                 "cafe",
	"coffee_shop":          "cafe",
	"bar":                  "bar",
	"pub":                  "bar",
	"pharmacy":             "pharmacy",
	"gas_station":          "fuel",
	"bakery":               "bakery",
	"hardware_store":       "hardware",
	"bank":                 "bank_atm",
	"atm":                  "bank_atm",
}

// OvertureCategory returns the app category for an Overture
// categories.primary value. Any category ending in "_restaurant" maps to
// restaurant, except fast_food_restaurant which maps to fast_food (plan's
// explicit exception). ok is false for an empty or unmapped category.
func OvertureCategory(primary string) (category string, ok bool) {
	if primary == "" {
		return "", false
	}
	if cat, mapped := overtureMap[primary]; mapped {
		return cat, true
	}
	if primary != "fast_food_restaurant" && strings.HasSuffix(primary, "_restaurant") {
		return "restaurant", true
	}
	return "", false
}

// NormalizeName lowercases name and strips everything but a-z0-9, for the
// dedupe rule in docs/WALKMAP.md.
func NormalizeName(name string) string {
	var b strings.Builder
	b.Grow(len(name))
	for _, r := range strings.ToLower(name) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}
