package taxonomy

import "testing"

func TestOSMCategory(t *testing.T) {
	cases := []struct {
		tags map[string]string
		want string
		ok   bool
	}{
		{map[string]string{"shop": "convenience"}, "convenience", true},
		{map[string]string{"shop": "kiosk"}, "convenience", true},
		{map[string]string{"shop": "supermarket"}, "supermarket", true},
		{map[string]string{"amenity": "restaurant"}, "restaurant", true},
		{map[string]string{"amenity": "fast_food"}, "fast_food", true},
		{map[string]string{"amenity": "pub"}, "bar", true},
		{map[string]string{"amenity": "atm"}, "bank_atm", true},
		{map[string]string{"shop": "doityourself"}, "hardware", true},
		{map[string]string{"shop": "clothes"}, "", false},
		{map[string]string{"amenity": "school"}, "", false},
		{map[string]string{}, "", false},
		// shop takes priority over amenity when both present.
		{map[string]string{"shop": "bakery", "amenity": "restaurant"}, "bakery", true},
	}
	for _, c := range cases {
		got, ok := OSMCategory(c.tags)
		if got != c.want || ok != c.ok {
			t.Errorf("OSMCategory(%v) = (%q, %v), want (%q, %v)", c.tags, got, ok, c.want, c.ok)
		}
	}
}

func TestOvertureCategory(t *testing.T) {
	cases := []struct {
		primary string
		want    string
		ok      bool
	}{
		{"convenience_store", "convenience", true},
		{"grocery_store", "supermarket", true},
		{"restaurant", "restaurant", true},
		{"seafood_restaurant", "restaurant", true},
		{"fast_food_restaurant", "fast_food", true},
		{"coffee_shop", "cafe", true},
		{"pub", "bar", true},
		{"gas_station", "fuel", true},
		{"", "", false},
		{"design_studio", "", false},
		{"hair_salon", "", false},
	}
	for _, c := range cases {
		got, ok := OvertureCategory(c.primary)
		if got != c.want || ok != c.ok {
			t.Errorf("OvertureCategory(%q) = (%q, %v), want (%q, %v)", c.primary, got, ok, c.want, c.ok)
		}
	}
}

func TestNormalizeName(t *testing.T) {
	cases := map[string]string{
		"Sol Shop #12": "solshop12",
		"  Café  ":     "caf",
		"ABC-Mart":     "abcmart",
		"":             "",
	}
	for in, want := range cases {
		if got := NormalizeName(in); got != want {
			t.Errorf("NormalizeName(%q) = %q, want %q", in, got, want)
		}
	}
}
