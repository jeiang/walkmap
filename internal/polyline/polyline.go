// Package polyline decodes Valhalla's encoded route shapes. Valhalla
// encodes shape as Google's polyline algorithm at precision 1e-6
// ("polyline6"), rather than the usual 1e-5.
package polyline

// Decode6 decodes a polyline6-encoded string into a slice of [lat, lon]
// points.
func Decode6(encoded string) [][2]float64 {
	var points [][2]float64
	var lat, lon int
	i := 0
	for i < len(encoded) {
		dlat, n := decodeValue(encoded, i)
		i += n
		dlon, n := decodeValue(encoded, i)
		i += n
		lat += dlat
		lon += dlon
		points = append(points, [2]float64{float64(lat) / 1e6, float64(lon) / 1e6})
	}
	return points
}

// decodeValue reads one varint-encoded, zigzag-delta value starting at
// offset i, returning the value and the number of bytes consumed.
func decodeValue(s string, i int) (value, consumed int) {
	shift, result := 0, 0
	start := i
	for {
		b := int(s[i]) - 63
		i++
		result |= (b & 0x1f) << shift
		shift += 5
		if b < 0x20 {
			break
		}
	}
	if result&1 != 0 {
		result = ^(result >> 1)
	} else {
		result = result >> 1
	}
	return result, i - start
}
