# Observed unencoded Builder body size

Before-fix test fails short12 versus actual5, chunked unknown versus actual5,
HEAD12 versus actual0. Without decoding or a preview cap, bodySize now uses bytes
actually read. Header Content-Length remains independently preserved. Automatic
Go gzip still keeps wire-body size unknown; decoded size stays13 in regression.
Capped bodies and explicit gzip retain their prior semantics, not inferred totals.

Complete requestbuilder/summary suites pass x64/x86. Guarded app tests after this
change pass amd6445.781s/38649.029s; guarded native builds pass both architectures.
Docker Linux CGO race passes summary1.026s/requestbuilder11.511s. No frontend code
changed since the passing state/build check. Native final386 stamp is
8c471359-plus-observed-bytes. New six-response file checks short5B/decoded5B and
header12, with gzip/large branches unchanged. Body screenshot shows5B/short.
The older response-matrix-readback.json and its native cases preserve before-fix
12B behavior. observed-size-readback.json pins the corrected source hash.
