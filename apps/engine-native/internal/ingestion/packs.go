package ingestion

// PackPin is provenance, not an imported capability grant. Compiled catalog
// composition alone assigns it after pack verification and conformance.
type PackPin struct {
	ID      string `json:"id"`
	Version string `json:"version"`
	SHA256  string `json:"sha256"`
	Origin  string `json:"origin"`
	Scope   string `json:"scope"`
}

// Extend builds a fresh immutable catalog. Canonical IDs cannot be shadowed;
// conflicting aliases need an explicit winner, never an implicit local override.
func (c *Catalog) Extend(added []ImporterDescriptor, schemas []Schema, analyzers []Analyzer, choices map[string]string) (*Catalog, error) {
	ds := []ImporterDescriptor{}
	allSchemas := []Schema{}
	allAnalyzers := []Analyzer{}
	for _, d := range c.descriptors {
		ds = append(ds, d)
	}
	ds = append(ds, added...)
	for _, s := range c.schemas {
		allSchemas = append(allSchemas, s)
	}
	allSchemas = append(allSchemas, schemas...)
	for _, a := range c.analyzers {
		allAnalyzers = append(allAnalyzers, a)
	}
	allAnalyzers = append(allAnalyzers, analyzers...)
	canonical := map[string]bool{}
	for _, d := range ds {
		if canonical[d.ID] {
			return nil, Problem("PACK_ALIAS_CONFLICT", "canonical importer IDs cannot be overridden")
		}
		canonical[d.ID] = true
	}
	aliases := map[string]map[string]bool{}
	for _, d := range ds {
		for _, a := range d.Aliases {
			if canonical[a] {
				return nil, Problem("PACK_ALIAS_CONFLICT", "alias cannot shadow a canonical importer ID")
			}
			if aliases[a] == nil {
				aliases[a] = map[string]bool{}
			}
			aliases[a][d.ID] = true
		}
	}
	used := map[string]bool{}
	for alias, owners := range aliases {
		if len(owners) > 1 {
			winner := choices[alias]
			if !owners[winner] {
				return nil, Problem("PACK_ALIAS_CONFLICT", "compare conflicting aliases and choose a canonical winner explicitly")
			}
			used[alias] = true
		}
	}
	// Retained owner choices become dormant when deactivation removes a
	// conflict. They grant no precedence until the same owners collide again.

	for i := range ds {
		kept := []string{}
		for _, a := range ds[i].Aliases {
			if !used[a] || choices[a] == ds[i].ID {
				kept = append(kept, a)
			}
		}
		ds[i].Aliases = kept
	}
	return NewCatalog(ds, allSchemas, allAnalyzers)
}
