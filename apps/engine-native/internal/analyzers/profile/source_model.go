package profile

import (
	"github.com/aurakimjh/archscope/apps/engine-native/internal/profilemodel"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
)

const SourceModelPayloadVersion = "0.3.0"

func modelPayload(set profilemodel.ProfileSet) string {
	if _, present, _ := set.SourceContract(); present {
		return SourceModelPayloadVersion
	}
	return ModelPayloadVersion
}
func SourceModelDescriptor() resultcontract.Descriptor {
	d := ModelDescriptor()
	d.Payload = resultcontract.Range{Min: SourceModelPayloadVersion, Before: "0.3.1"}
	return d
}
