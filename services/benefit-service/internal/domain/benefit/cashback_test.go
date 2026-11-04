package benefit

import (
	"errors"
	"math"
	"testing"
)

func TestCalculateCashback(t *testing.T) {
	tests := []struct {
		name           string
		payableKopecks int64
		rateBPS        int64
		want           int64
		wantErr        error
	}{
		{
			name:           "one_percent",
			payableKopecks: 100_000,
			rateBPS:        100,
			want:           1_000,
		},
		{
			name:           "three_percent",
			payableKopecks: 100_000,
			rateBPS:        300,
			want:           3_000,
		},
		{
			name:           "five_percent",
			payableKopecks: 100_000,
			rateBPS:        500,
			want:           5_000,
		},
		{
			name:           "fractional_kopeck_is_rounded_down",
			payableKopecks: 199_999,
			rateBPS:        300,
			// Exact cashback is 5999.97 kopecks; discard the fractional part.
			want: 5_999,
		},
		{
			name:           "one_bps_below_one_kopeck",
			payableKopecks: 9_999,
			rateBPS:        1,
			want:           0,
		},
		{
			name:           "one_bps_at_one_kopeck",
			payableKopecks: 10_000,
			rateBPS:        1,
			want:           1,
		},
		{
			name:           "zero_amount",
			payableKopecks: 0,
			rateBPS:        300,
			want:           0,
		},
		{
			name:           "zero_rate",
			payableKopecks: 100_000,
			rateBPS:        0,
			want:           0,
		},
		{
			name:           "one_hundred_percent",
			payableKopecks: 100_000,
			rateBPS:        10_000,
			want:           100_000,
		},
		{
			name:           "maximum_amount_at_one_hundred_percent",
			payableKopecks: math.MaxInt64,
			rateBPS:        10_000,
			// This catches overflow from multiplying the full amount by the rate.
			want: math.MaxInt64,
		},
		{
			name:           "negative_amount",
			payableKopecks: -1,
			rateBPS:        300,
			want:           0,
			wantErr:        ErrInvalidAmount,
		},
		{
			name:           "negative_rate",
			payableKopecks: 100_000,
			rateBPS:        -1,
			want:           0,
			wantErr:        ErrInvalidRate,
		},
		{
			name:           "rate_above_one_hundred_percent",
			payableKopecks: 100_000,
			rateBPS:        10_001,
			want:           0,
			wantErr:        ErrInvalidRate,
		},
		{
			name:           "zero_amount_does_not_allow_invalid_rate",
			payableKopecks: 0,
			rateBPS:        -1,
			want:           0,
			wantErr:        ErrInvalidRate,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := CalculateCashback(tc.payableKopecks, tc.rateBPS)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("CalculateCashback(%d, %d) error = %v, want %v",
					tc.payableKopecks, tc.rateBPS, err, tc.wantErr)
			}
			if got != tc.want {
				t.Errorf("CalculateCashback(%d, %d) = %d kopecks, want %d",
					tc.payableKopecks, tc.rateBPS, got, tc.want)
			}
		})
	}
}
