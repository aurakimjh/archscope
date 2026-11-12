package profile

import (
	"github.com/aurakimjh/archscope/apps/engine-native/internal/profilemodel"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
)

const SourceModelPayloadVersion = "0.4.0"
const LegacySourceModelPayloadVersion = "0.3.0"

func modelPayload(set profilemodel.ProfileSet) string {
	if c, present, _ := set.SourceContract(); present {
		if c.Revision == profilemodel.LegacySourceNormalizer {
			return LegacySourceModelPayloadVersion
		}
		return SourceModelPayloadVersion
	}
	return ModelPayloadVersion
}
func SourceModelDescriptor() resultcontract.Descriptor {
	d := ModelDescriptor()
	d.Payload = resultcontract.Range{Min: SourceModelPayloadVersion, Before: "0.4.1"}
	return d
}

func LegacySourceModelDescriptor() resultcontract.Descriptor {
	d := ModelDescriptor()
	d.Payload = resultcontract.Range{Min: LegacySourceModelPayloadVersion, Before: "0.3.1"}
	d.Operations = []string{"display", "save", "compute", "json", "csv", "html", "report-pack"}
	return d
}
