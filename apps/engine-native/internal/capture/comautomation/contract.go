package comautomation

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"strings"
)

// API identifiers/types are interoperability facts from the bundled reference
// declarations. Vendor binaries/source are not redistributed with ArchScope.
//
//go:embed reference_contract.json
var reference embed.FS
var referenceContracts struct {
	Interfaces map[string]struct {
		IID     string           `json:"iid"`
		Base    string           `json:"base"`
		Methods []abiMethod      `json:"methods"`
		IDs     map[string]int32 `json:"dispatchIds"`
	} `json:"interfaces"`
	LogTypes map[string]string `json:"logTypes"`
}

func init() {
	data, err := reference.ReadFile("reference_contract.json")
	if err != nil {
		panic(err)
	}
	if err = json.Unmarshal(data, &referenceContracts); err != nil {
		panic(err)
	}
}

type Typed struct {
	Name   string
	Object Object
}

func (t Typed) Members() []string { return t.Object.Members() }
func (t Typed) Invoke(ctx context.Context, name string, args []any, put bool) (any, error) {
	return t.Object.Invoke(ctx, name, args, put)
}
func (t Typed) Contract() string     { return t.Name }
func (*Controller) Contract() string { return "IHTTPAnalyzerStandAlone" }
func dispatchIDs(object Object) map[string]int32 {
	result := map[string]int32{}
	if typed, ok := object.(interface{ Contract() string }); ok {
		for name, id := range referenceContracts.Interfaces[typed.Contract()].IDs {
			result[strings.ToLower(name)] = id
		}
	}
	for index, name := range object.Members() {
		key := strings.ToLower(name)
		if _, exists := result[key]; !exists {
			result[key] = int32(65536 + index)
		}
		if key == "item" {
			result[key] = 0
		}
		if key == "_newenum" {
			result[key] = -4
		}
	}
	return result
}

type comDate float64
type unavailable string

func (u unavailable) Error() string { return string(u) }
func namedLogType(value any) (string, error) {
	if text, ok := value.(string); ok {
		return strings.ToLower(text), nil
	}
	number, err := integer(value)
	if err != nil {
		return "", err
	}
	name, ok := referenceContracts.LogTypes[fmt.Sprint(number)]
	if !ok {
		return "", fmt.Errorf("unknown reference export enum")
	}
	return name, nil
}

// abiMethod records API shape and vtable order, not vendor implementation.
type abiParam struct {
	Name string `json:"name"`
	Type string `json:"type"`
}
type abiMethod struct {
	Name    string     `json:"name"`
	Kind    string     `json:"kind"`
	Returns string     `json:"returns"`
	Params  []abiParam `json:"params"`
}

func interfaceMethods(name string) []abiMethod {
	contract, ok := referenceContracts.Interfaces[name]
	if !ok {
		return nil
	}
	var methods []abiMethod
	if contract.Base != "" && contract.Base != "IDispatch" {
		methods = interfaceMethods(contract.Base)
	}
	methods = append(methods, contract.Methods...)
	if name == "IHTTPAnalyzerStandAlone" {
		for _, property := range []struct {
			name, kind string
			write      bool
		}{{"Backend", "WideString", true}, {"SessionID", "WideString", false}, {"ProxyAddress", "WideString", false}, {"BuildCommit", "WideString", false}, {"CaptureBodies", "WordBool", true}} {
			methods = append(methods, abiMethod{Name: property.name, Kind: "get", Returns: property.kind})
			if property.write {
				methods = append(methods, abiMethod{Name: property.name, Kind: "put", Returns: "void", Params: []abiParam{{Name: "Value", Type: property.kind}}})
			}
		}
	}
	return methods
}
