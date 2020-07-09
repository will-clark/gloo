package transformation_test

import (
	envoyroute "github.com/envoyproxy/go-control-plane/envoy/api/v2/route"
	"github.com/envoyproxy/go-control-plane/pkg/conversion"
	structpb "github.com/golang/protobuf/ptypes/struct"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	envoytransformation "github.com/solo-io/gloo/projects/gloo/pkg/api/external/envoy/extensions/transformation"
	v1 "github.com/solo-io/gloo/projects/gloo/pkg/api/v1"
	"github.com/solo-io/gloo/projects/gloo/pkg/api/v1/options/transformation"
	"github.com/solo-io/gloo/projects/gloo/pkg/plugins"
	. "github.com/solo-io/gloo/projects/gloo/pkg/plugins/transformation"
)

var _ = Describe("Plugin", func() {
	var (
		p        *Plugin
		t        *transformation.Transformations
		expected *structpb.Struct
	)

	BeforeEach(func() {
		p = NewPlugin()
		t = &transformation.Transformations{
			ClearRouteCache: true,
		}
		e := &envoytransformation.RouteTransformations{
			ClearRouteCache: true,
			Transformations: []*envoytransformation.RouteTransformations_RouteTransformation{
				{
					Match: &envoytransformation.RouteTransformations_RouteTransformation_RequestMatch_{
						RequestMatch: &envoytransformation.RouteTransformations_RouteTransformation_RequestMatch{
							ClearRouteCache: true,
						},
					},
				},
			},
		}
		configStruct, err := conversion.MessageToStruct(e)
		Expect(err).NotTo(HaveOccurred())

		expected = configStruct
	})

	It("sets transformation config for weighted destinations", func() {
		out := &envoyroute.WeightedCluster_ClusterWeight{}
		err := p.ProcessWeightedDestination(plugins.RouteParams{}, &v1.WeightedDestination{
			Options: &v1.WeightedDestinationOptions{
				Transformations: t,
			},
		}, out)
		Expect(err).NotTo(HaveOccurred())
		Expect(out.PerFilterConfig).To(HaveKeyWithValue(FilterName, expected))
	})
	It("sets transformation config for virtual hosts", func() {
		out := &envoyroute.VirtualHost{}
		err := p.ProcessVirtualHost(plugins.VirtualHostParams{}, &v1.VirtualHost{
			Options: &v1.VirtualHostOptions{
				Transformations: t,
			},
		}, out)
		Expect(err).NotTo(HaveOccurred())
		Expect(out.PerFilterConfig).To(HaveKeyWithValue(FilterName, expected))
	})
	It("sets transformation config for routes", func() {
		out := &envoyroute.Route{}
		err := p.ProcessRoute(plugins.RouteParams{}, &v1.Route{
			Options: &v1.RouteOptions{
				Transformations: t,
			},
		}, out)
		Expect(err).NotTo(HaveOccurred())
		Expect(out.PerFilterConfig).To(HaveKeyWithValue(FilterName, expected))
	})
	It("sets filters correctly when early filters exist", func() {
		out := &envoyroute.Route{}
		err := p.ProcessRoute(plugins.RouteParams{}, &v1.Route{
			Options: &v1.RouteOptions{
				StagedTransformations: &transformation.TransformationStages{
					Early: &transformation.RequestResponseTransformations{RequestTransforms: []*transformation.RequestMatch{{}}},
				},
			},
		}, out)
		filters, err := p.HttpFilters(plugins.Params{}, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(len(filters)).To(Equal(2))
		// not empty as stage is 1; TODO: deserialize and verify that.
		value := filters[0].HttpFilter.GetTypedConfig().GetValue()
		Expect(value).NotTo(BeEmpty())
	})
	It("sets filters correctly when no early filters exist", func() {
		filters, err := p.HttpFilters(plugins.Params{}, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(len(filters)).To(Equal(1))
		value := filters[0].HttpFilter.GetTypedConfig().GetValue()
		Expect(value).To(BeEmpty())
	})
})
