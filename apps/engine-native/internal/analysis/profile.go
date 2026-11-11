package analysis

import (
	"context"
	"fmt"

	analyzer "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/profile"
	profileparser "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/profile"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/profilemodel"
)

func withProfile[T any](s *Service, parent context.Context, handle, operation string, run func(context.Context, analyzer.CoreEvidence) (T, error)) (out T, failure error) {
	ctx, finish, e := s.beginLoad(parent)
	if e != nil {
		return out, e
	}
	defer finish()
	_, result, e := s.Read(handle)
	if e != nil {
		return out, e
	}
	if verdict := s.results.Check(result, operation); !verdict.Allowed {
		return out, fmt.Errorf("%s", verdict.Reason)
	}
	core, e := analyzer.ReadCore(result)
	if e != nil {
		return out, e
	}
	if e = ctx.Err(); e != nil {
		return out, e
	}
	return run(ctx, core)
}
func (s *Service) ProfileTree(ctx context.Context, handle string) (profilemodel.Tree, error) {
	return withProfile(s, ctx, handle, "compute", func(ctx context.Context, core analyzer.CoreEvidence) (profilemodel.Tree, error) {
		return profilemodel.BuildTree(ctx, core.Model, core.Selection)
	})
}
func (s *Service) ProfileDrilldown(ctx context.Context, handle string, filters []profilemodel.PathFilter) ([]profilemodel.Stage, error) {
	return withProfile(s, ctx, handle, "compute", func(ctx context.Context, core analyzer.CoreEvidence) ([]profilemodel.Stage, error) {
		return profilemodel.Drilldown(ctx, core.Model, core.Selection, filters)
	})
}
func (s *Service) ProfilePprof(ctx context.Context, handle, tickScale string) ([]byte, error) {
	return withProfile(s, ctx, handle, "compute", func(ctx context.Context, core analyzer.CoreEvidence) ([]byte, error) {
		return profilemodel.ExportPprof(ctx, core.Model, core.Selection, tickScale)
	})
}
func (s *Service) ProfileCompare(ctx context.Context, before, after string) (profilemodel.Difference, error) {
	return withProfile(s, ctx, before, "compare", func(ctx context.Context, left analyzer.CoreEvidence) (profilemodel.Difference, error) {
		_, result, e := s.Read(after)
		if e != nil {
			return profilemodel.Difference{}, e
		}
		if verdict := s.results.Check(result, "compare"); !verdict.Allowed {
			return profilemodel.Difference{}, fmt.Errorf("%s", verdict.Reason)
		}
		right, e := analyzer.ReadCore(result)
		if e != nil {
			return profilemodel.Difference{}, e
		}
		return profilemodel.Compare(ctx, left.Model, left.Selection, right.Model, right.Selection)
	})
}

func (s *Service) NormalizeLegacyProfileSet(parent context.Context, path, format string) (profilemodel.ProfileSet, error) {
	ctx, finish, e := s.beginLoad(parent)
	if e != nil {
		return profilemodel.ProfileSet{}, e
	}
	defer finish()
	set, _, e := profileparser.ParseLegacySet(ctx, path, format)
	return set, e
}
