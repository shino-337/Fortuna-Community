package listlimit

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClamp(t *testing.T) {
	cases := []struct {
		raw      string
		def, max int
		want     int
	}{
		{"", 50, 200, 50},
		{"10", 50, 200, 10},
		{"200", 50, 200, 200},
		{"201", 50, 200, 200},
		{"0", 50, 200, 50},
		{"-1", 50, 200, 50},
		{"abc", 50, 200, 50},
		{"", 500, 200, 200}, // default never exceeds the hard max
	}
	for _, tc := range cases {
		require.Equal(t, tc.want, Clamp(tc.raw, tc.def, tc.max), tc.raw)
	}
}

func TestTrim(t *testing.T) {
	rows, cut := Trim([]int{1, 2, 3}, 2)
	require.Equal(t, []int{1, 2}, rows)
	require.True(t, cut)
	rows, cut = Trim([]int{1, 2}, 2)
	require.Equal(t, []int{1, 2}, rows)
	require.False(t, cut)
}
