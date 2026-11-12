package inference

import (
	"math/big"
	"testing"
)

func TestR17ExactScaledRateParsing(t *testing.T) {
	v, e := parseExactRate("20/3")
	if e != nil {
		t.Fatal(e)
	}
	if got := new(big.Rat).Mul(v, big.NewRat(1048576, 1)).RatString(); got != "20971520/3" {
		t.Fatal(got)
	}
	for _, bad := range []string{"1/0", "NaN", "1/2/3", "-1/3"} {
		if _, e := parseExactRate(bad); e == nil {
			t.Fatal(bad)
		}
	}
}
