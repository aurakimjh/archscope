package analysis

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	analyzer "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/profile"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/profilemodel"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/wirejson"
)

type ProfileInventory struct {
	Format       string                      `json:"format"`
	ResultSHA256 string                      `json:"result_sha256"`
	Selection    profilemodel.Selection      `json:"selection"`
	Profiles     []profilemodel.Profile      `json:"profiles"`
	Metrics      []profilemodel.Metric       `json:"metrics"`
	Policy       *profilemodel.CapturePolicy `json:"capture_policy"`
}

func (s *Service) ProfileInventory(ctx context.Context, handle string) (ProfileInventory, error) {
	return withProfile(s, ctx, handle, "compute", func(ctx context.Context, core analyzer.CoreEvidence) (ProfileInventory, error) {
		body, _, e := s.Read(handle)
		sum := sha256.Sum256(body)
		out := ProfileInventory{Format: core.Model.Format, ResultSHA256: hex.EncodeToString(sum[:]), Selection: core.Selection, Metrics: core.Model.Metrics, Profiles: []profilemodel.Profile{}, Policy: core.Model.CapturePolicy(core.Selection.MetricID)}
		for _, p := range core.Model.Profiles {
			p.Samples = nil
			out.Profiles = append(out.Profiles, p)
		}
		return out, e
	})
}
func (s *Service) QueryProfile(ctx context.Context, handle, resultSHA string, q profilemodel.Query) (profilemodel.QueryPage, error) {
	return withProfile(s, ctx, handle, "compute", func(ctx context.Context, core analyzer.CoreEvidence) (profilemodel.QueryPage, error) {
		body, _, e := s.Read(handle)
		if e != nil {
			return profilemodel.QueryPage{}, e
		}
		sum := sha256.Sum256(body)
		if hex.EncodeToString(sum[:]) != resultSHA {
			return profilemodel.QueryPage{}, fmt.Errorf("PROFILE_RESULT_STALE")
		}
		return profilemodel.QuerySet(ctx, core.Model, resultSHA, q)
	})
}
func DecodeProfileQuery(body []byte) (profilemodel.Query, error) {
	var q profilemodel.Query
	if len(body) > 64<<10 {
		return q, fmt.Errorf("PROFILE_QUERY_INVALID")
	}
	if e := decodeQueryStrict(body, &q); e != nil {
		return q, e
	}
	return q, nil
}
func decodeQueryStrict(body []byte, out any) error {
	var raw json.RawMessage
	if e := json.Unmarshal(body, &raw); e != nil {
		return e
	}
	return strictQueryJSON(body, out)
}

func strictQueryJSON(body []byte, out any) error {
	if e := wirejson.Decode(body, new(any)); e != nil {
		return e
	}
	d := json.NewDecoder(bytes.NewReader(body))
	d.DisallowUnknownFields()
	d.UseNumber()
	return d.Decode(out)
}

func (s *Service) MaterializeProfileSelection(ctx context.Context, handle, digest string, selection profilemodel.Selection) (ResultView, ResultLease, error) {
	var view ResultView
	var lease ResultLease
	core, e := withProfile(s, ctx, handle, "compute", func(ctx context.Context, c analyzer.CoreEvidence) (analyzer.CoreEvidence, error) {
		body, _, e := s.Read(handle)
		if e != nil {
			return c, e
		}
		if bytesDigest(body) != digest {
			return c, fmt.Errorf("PROFILE_RESULT_STALE")
		}
		c.Selection = profilemodel.CopySelection(selection)
		return c, c.Model.ValidateSelection(selection)
	})
	if e != nil {
		return view, lease, e
	}
	result, e := analyzer.BuildSet(ctx, core.Model, core.Selection, "explicit selected profile view")
	if e != nil {
		return view, lease, e
	}
	body, e := json.Marshal(result)
	if e != nil {
		return view, lease, e
	}
	return s.LoadOwnedSnapshot(ctx, body)
}
