package visuals

import (
	_ "embed"
	"encoding/json"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/ingestion"
)

//go:embed assets/visual-artifact-spec.schema.json
var SpecSchema []byte
var validateSpecSchema = func() func(json.RawMessage) error {
	validate, e := ingestion.CompileSchema(SpecSchema)
	if e != nil {
		panic(e)
	}
	return validate
}()

//go:embed assets/font-OFL.txt
var FontLicense []byte
