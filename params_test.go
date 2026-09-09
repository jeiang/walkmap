package main

import "testing"

func TestParseMinutes(t *testing.T) {
	cases := []struct {
		in      string
		want    int
		wantErr bool
	}{
		{"", 20, false},
		{"5", 5, false},
		{"90", 90, false},
		{"45", 45, false},
		{"4", 0, true},
		{"91", 0, true},
		{"abc", 0, true},
	}
	for _, c := range cases {
		got, err := parseMinutes(c.in)
		if (err != nil) != c.wantErr {
			t.Errorf("parseMinutes(%q) err = %v, wantErr %v", c.in, err, c.wantErr)
			continue
		}
		if !c.wantErr && got != c.want {
			t.Errorf("parseMinutes(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestParseMinConfidence(t *testing.T) {
	cases := []struct {
		in      string
		want    float64
		wantErr bool
	}{
		{"", 0.5, false},
		{"0", 0, false},
		{"1", 1, false},
		{"0.75", 0.75, false},
		{"-0.1", 0, true},
		{"1.1", 0, true},
		{"abc", 0, true},
	}
	for _, c := range cases {
		got, err := parseMinConfidence(c.in)
		if (err != nil) != c.wantErr {
			t.Errorf("parseMinConfidence(%q) err = %v, wantErr %v", c.in, err, c.wantErr)
			continue
		}
		if !c.wantErr && got != c.want {
			t.Errorf("parseMinConfidence(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestParseLatLon(t *testing.T) {
	cases := []struct {
		lat, lon string
		wantErr  bool
	}{
		{"13.0975", "-59.6165", false},
		{"", "-59.6165", true},
		{"13.0975", "", true},
		{"91", "0", true},
		{"0", "181", true},
		{"abc", "0", true},
	}
	for _, c := range cases {
		_, _, err := parseLatLon(c.lat, c.lon)
		if (err != nil) != c.wantErr {
			t.Errorf("parseLatLon(%q, %q) err = %v, wantErr %v", c.lat, c.lon, err, c.wantErr)
		}
	}
}

func TestParseLimit(t *testing.T) {
	cases := []struct {
		in       string
		def, max int
		want     int
		wantErr  bool
	}{
		{"", 20, 100, 20, false},
		{"5", 20, 100, 5, false},
		{"1000", 20, 100, 100, false},
		{"0", 20, 100, 0, true},
		{"abc", 20, 100, 0, true},
	}
	for _, c := range cases {
		got, err := parseLimit(c.in, c.def, c.max)
		if (err != nil) != c.wantErr {
			t.Errorf("parseLimit(%q) err = %v, wantErr %v", c.in, err, c.wantErr)
			continue
		}
		if !c.wantErr && got != c.want {
			t.Errorf("parseLimit(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}
