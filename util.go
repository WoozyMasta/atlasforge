// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/atlasforge

package atlasforge

// nextPowerOfTwo returns the smallest power of two >= n.
func nextPowerOfTwo(n int) int {
	if n <= 0 {
		return 1
	}
	if n&(n-1) == 0 {
		return n
	}

	p := 1
	for p < n {
		p <<= 1
	}

	return p
}

// absPowerDiff returns absolute difference between power-of-two exponents.
func absPowerDiff(a, b int) int {
	pa := powerOfTwoCeil(a)
	pb := powerOfTwoCeil(b)
	d := pa - pb
	if d < 0 {
		return -d
	}

	return d
}

// powerOfTwoCeil returns exponent k for the smallest 2^k >= n.
func powerOfTwoCeil(n int) int {
	if n <= 0 {
		return 0
	}

	p := 0
	v := 1
	for v < n {
		v <<= 1
		p++
	}

	return p
}

// removeAt removes slice element by index.
func removeAt[T any](s []T, i int) []T {
	if i < 0 || i >= len(s) {
		return s
	}

	copy(s[i:], s[i+1:])
	return s[:len(s)-1]
}

// removeAtSwap removes element by index without preserving order.
func removeAtSwap[T any](s []T, i int) []T {
	if i < 0 || i >= len(s) {
		return s
	}

	last := len(s) - 1
	s[i] = s[last]

	return s[:last]
}
