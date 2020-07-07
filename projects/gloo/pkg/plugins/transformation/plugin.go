package transformation

import (
	"context"

	envoyroute "github.com/envoyproxy/go-control-plane/envoy/api/v2/route"
	"github.com/gogo/protobuf/types"
	wrappers "github.com/golang/protobuf/ptypes/wrappers"

	"github.com/solo-io/gloo/pkg/utils/regexutils"
	envoyroutev3 "github.com/solo-io/gloo/projects/gloo/pkg/api/external/envoy/config/route/v3"
	v3 "github.com/solo-io/gloo/projects/gloo/pkg/api/external/envoy/type/matcher/v3"
	"github.com/solo-io/gloo/projects/gloo/pkg/api/v1/core/matchers"
	transformation "github.com/solo-io/gloo/projects/gloo/pkg/api/v1/options/transformation"

	envoytransformation "github.com/solo-io/gloo/projects/gloo/pkg/api/external/envoy/extensions/transformation"
	v1 "github.com/solo-io/gloo/projects/gloo/pkg/api/v1"
	"github.com/solo-io/gloo/projects/gloo/pkg/bootstrap"
	"github.com/solo-io/gloo/projects/gloo/pkg/plugins"
	"github.com/solo-io/gloo/projects/gloo/pkg/plugins/pluginutils"
)

const (
	FilterName       = "io.solo.transformation"
	EarlyStageNumber = 1
)

var (
	earlyPluginStage = plugins.AfterStage(plugins.FaultStage)
	pluginStage      = plugins.AfterStage(plugins.AuthZStage)
)

type Plugin struct {
	RequireTransformationFilter bool
}

func NewPlugin() *Plugin {
	return &Plugin{}
}

func (p *Plugin) Init(params plugins.InitParams) error {
	p.RequireTransformationFilter = false
	return nil
}

// TODO(yuval-k): We need to figure out what\if to do in edge cases where there is cluster weight transform
func (p *Plugin) ProcessVirtualHost(params plugins.VirtualHostParams, in *v1.VirtualHost, out *envoyroute.VirtualHost) error {
	envoyTransformation := convertTransformation(params.Ctx, in.GetOptions().GetTransformations(), in.GetOptions().GetEarlyTransformations())
	if envoyTransformation == nil {
		return nil
	}
	p.RequireTransformationFilter = true
	err := validateTransformation(params.Ctx, envoyTransformation)
	if err != nil {
		return err
	}

	p.RequireTransformationFilter = true
	return pluginutils.SetVhostPerFilterConfig(out, FilterName, envoyTransformation)
}

func (p *Plugin) ProcessRoute(params plugins.RouteParams, in *v1.Route, out *envoyroute.Route) error {
	envoyTransformation := convertTransformation(params.Ctx, in.GetOptions().GetTransformations(), in.GetOptions().GetEarlyTransformations())
	if envoyTransformation == nil {
		return nil
	}
	p.RequireTransformationFilter = true
	err := validateTransformation(params.Ctx, envoyTransformation)
	if err != nil {
		return err
	}

	p.RequireTransformationFilter = true
	return pluginutils.SetRoutePerFilterConfig(out, FilterName, envoyTransformation)
}

func (p *Plugin) ProcessWeightedDestination(params plugins.RouteParams, in *v1.WeightedDestination, out *envoyroute.WeightedCluster_ClusterWeight) error {
	envoyTransformation := convertTransformation(params.Ctx, in.GetOptions().GetTransformations(), in.GetOptions().GetEarlyTransformations())
	if envoyTransformation == nil {
		return nil
	}

	p.RequireTransformationFilter = true
	err := validateTransformation(params.Ctx, envoyTransformation)
	if err != nil {
		return err
	}

	return pluginutils.SetWeightedClusterPerFilterConfig(out, FilterName, envoyTransformation)
}

func (p *Plugin) HttpFilters(params plugins.Params, listener *v1.HttpListener) ([]plugins.StagedHttpFilter, error) {
	earlyStageConfig := &envoytransformation.FilterTransformations{
		Stage: EarlyStageNumber,
	}
	earlyFilter, err := plugins.NewStagedFilterWithConfig(FilterName, earlyStageConfig, earlyPluginStage)
	if err != nil {
		return nil, err
	}
	return []plugins.StagedHttpFilter{
		earlyFilter,
		plugins.NewStagedFilter(FilterName, pluginStage),
	}, nil
}

func convertTransformation(ctx context.Context, t *transformation.Transformations, et *transformation.EarlyTransformations) *envoytransformation.RouteTransformations {
	if t == nil && et == nil {
		return nil
	}

	ret := &envoytransformation.RouteTransformations{}
	if t != nil {
		// keep deprecated config until we are sure we don't need it.
		// on newer envoys it will be ignored.
		ret.RequestTransformation = t.RequestTransformation
		ret.ClearRouteCache = t.ClearRouteCache
		ret.ResponseTransformation = t.ResponseTransformation
		// new config:
		// we have to have it too, as if any new config is defined the deprecated config is ignored.
		ret.Transformations = append(ret.Transformations, &envoytransformation.RouteTransformations_RouteTransformation{
			Match: &envoytransformation.RouteTransformations_RouteTransformation_RequestMatch_{
				RequestMatch: &envoytransformation.RouteTransformations_RouteTransformation_RequestMatch{
					Match:                  nil,
					RequestTransformation:  t.RequestTransformation,
					ClearRouteCache:        t.ClearRouteCache,
					ResponseTransformation: t.ResponseTransformation,
				},
			},
		})
	}

	for _, earlyRespTransform := range et.GetResponseTransforms() {
		ret.Transformations = append(ret.Transformations, &envoytransformation.RouteTransformations_RouteTransformation{
			Stage: EarlyStageNumber,
			Match: &envoytransformation.RouteTransformations_RouteTransformation_ResponseMatch_{
				ResponseMatch: &envoytransformation.RouteTransformations_RouteTransformation_ResponseMatch{
					Match:                  getResponseMatcher(ctx, earlyRespTransform),
					ResponseTransformation: earlyRespTransform.ResponseTransformation,
				},
			},
		})
	}

	return ret
}

func validateTransformation(ctx context.Context, transformations *envoytransformation.RouteTransformations) error {
	err := bootstrap.ValidateBootstrap(ctx, bootstrap.BuildPerFilterBootstrapYaml(FilterName, transformations))
	if err != nil {
		return err
	}
	return nil
}

func getResponseMatcher(ctx context.Context, m *transformation.ResponseMatch) *envoytransformation.ResponseMatcher {
	matcher := &envoytransformation.ResponseMatcher{
		Headers: envoyHeaderMatcher(ctx, m.Matchers),
	}
	if m.ResponseCodeDetails != "" {
		matcher.ResponseCodeDetails = &v3.StringMatcher{
			MatchPattern: &v3.StringMatcher_Exact{Exact: m.ResponseCodeDetails},
		}
	}
	return matcher
}

func convertUint32(i *wrappers.UInt32Value) *types.UInt32Value {
	if i == nil {
		return nil
	}
	return &types.UInt32Value{
		Value: i.Value,
	}
}

func envoyHeaderMatcher(ctx context.Context, in []*matchers.HeaderMatcher) []*envoyroutev3.HeaderMatcher {
	var out []*envoyroutev3.HeaderMatcher
	for _, matcher := range in {
		envoyMatch := &envoyroutev3.HeaderMatcher{
			Name: matcher.Name,
		}
		if matcher.Value == "" {
			envoyMatch.HeaderMatchSpecifier = &envoyroutev3.HeaderMatcher_PresentMatch{
				PresentMatch: true,
			}
		} else {
			if matcher.Regex {
				regex := regexutils.NewRegex(ctx, matcher.Value)
				envoyMatch.HeaderMatchSpecifier = &envoyroutev3.HeaderMatcher_SafeRegexMatch{
					SafeRegexMatch: &v3.RegexMatcher{
						EngineType: &v3.RegexMatcher_GoogleRe2{GoogleRe2: &v3.RegexMatcher_GoogleRE2{MaxProgramSize: convertUint32(regex.GetGoogleRe2().GetMaxProgramSize())}},
						Regex:      regex.Regex,
					},
				}
			} else {
				envoyMatch.HeaderMatchSpecifier = &envoyroutev3.HeaderMatcher_ExactMatch{
					ExactMatch: matcher.Value,
				}
			}
		}

		if matcher.InvertMatch {
			envoyMatch.InvertMatch = true
		}
		out = append(out, envoyMatch)
	}
	return out
}
