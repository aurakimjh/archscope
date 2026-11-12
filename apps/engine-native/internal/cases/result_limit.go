package cases

import "fmt"

// ResultSizeError reports the measured size, without parsing an oversized result
// or dropping observations to make it fit. Sample count is unknown at this boundary.
func ResultSizeError(actual int64) error {
	return problem("CASE_BUDGET_EXCEEDED", fmt.Sprintf("result is %d bytes; case result limit is %d bytes (64 MiB); sample count unavailable before bounded decode. Select fewer profiles/metrics or shorten the capture and create a new result; this result cannot be saved or pinned as a baseline", actual, MaxObjectBytes))
}
