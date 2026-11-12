package profilemodel

import (
	"context"
	"math/big"
)

// VisitEvidence exposes the same canonical population to detail indexes without
// reimplementing selection or inferring chronology from source text.
func VisitEvidence(ctx context.Context, set ProfileSet, selection Selection, visit func(Profile, Sample, []string, *string) error) error {
	return set.each(ctx, selection, func(p Profile, s Sample, path []string, _ *big.Rat) error {
		var timestamp *string
		if s.Timestamp != nil {
			at, e := Number(*s.Timestamp)
			if e != nil {
				return e
			}
			scale, e := Number(p.TimeScale)
			if e != nil {
				return e
			}
			text := Text(new(big.Rat).Mul(at, scale))
			timestamp = &text
		}
		return visit(p, s, path, timestamp)
	})
}
