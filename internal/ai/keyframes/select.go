package keyframes

// Select returns up to max evenly spaced keys, always including the first and
// last when max >= 2. Order is preserved (timeline order).
func Select(keys []string, max int) []string {
	n := len(keys)
	if n == 0 || max <= 0 {
		return nil
	}
	if n <= max {
		out := make([]string, n)
		copy(out, keys)
		return out
	}
	if max == 1 {
		return []string{keys[0]}
	}

	out := make([]string, 0, max)
	seen := make(map[int]struct{}, max)
	for i := 0; i < max; i++ {
		idx := i * (n - 1) / (max - 1)
		if _, ok := seen[idx]; ok {
			continue
		}
		seen[idx] = struct{}{}
		out = append(out, keys[idx])
	}
	return out
}
