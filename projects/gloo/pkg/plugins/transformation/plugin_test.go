package transformation_test

import (
	envoyroute "github.com/envoyproxy/go-control-plane/envoy/api/v2/route"
	"github.com/golang/protobuf/ptypes/any"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	envoytransformation "github.com/solo-io/gloo/projects/gloo/pkg/api/external/envoy/extensions/transformation"
	v1 "github.com/solo-io/gloo/projects/gloo/pkg/api/v1"
	"github.com/solo-io/gloo/projects/gloo/pkg/api/v1/options/transformation"
	"github.com/solo-io/gloo/projects/gloo/pkg/plugins"
	. "github.com/solo-io/gloo/projects/gloo/pkg/plugins/transformation"
	"github.com/solo-io/gloo/projects/gloo/pkg/utils"
)

var _ = Describe("Plugin", func() {
	var (
		p               *Plugin
		expected        *any.Any
		outputTransform *envoytransformation.RouteTransformations
	)

	Context("deprecated transformations", func() {
		var (
			inputTransform *transformation.Transformations
		)
		BeforeEach(func() {
			p = NewPlugin()
			inputTransform = &transformation.Transformations{
				ClearRouteCache: true,
			}
			outputTransform = &envoytransformation.RouteTransformations{
				// deperecated config gets old and new config
				ClearRouteCache: true,
				Transformations: []*envoytransformation.RouteTransformations_RouteTransformation{
					{
						Match: &envoytransformation.RouteTransformations_RouteTransformation_RequestMatch_{
							RequestMatch: &envoytransformation.RouteTransformations_RouteTransformation_RequestMatch{ClearRouteCache: true},
						},
					},
				},
			}
			configStruct, err := utils.MessageToAny(outputTransform)
			Expect(err).NotTo(HaveOccurred())

			expected = configStruct
		})

		It("sets transformation config for weighted destinations", func() {
			out := &envoyroute.WeightedCluster_ClusterWeight{}
			err := p.ProcessWeightedDestination(plugins.RouteParams{}, &v1.WeightedDestination{
				Options: &v1.WeightedDestinationOptions{
					Transformations: inputTransform,
				},
			}, out)
			Expect(err).NotTo(HaveOccurred())
			Expect(out.TypedPerFilterConfig).To(HaveKeyWithValue(FilterName, expected))
		})
		It("sets transformation config for virtual hosts", func() {
			out := &envoyroute.VirtualHost{}
			err := p.ProcessVirtualHost(plugins.VirtualHostParams{}, &v1.VirtualHost{
				Options: &v1.VirtualHostOptions{
					Transformations: inputTransform,
				},
			}, out)
			Expect(err).NotTo(HaveOccurred())
			Expect(out.TypedPerFilterConfig).To(HaveKeyWithValue(FilterName, expected))
		})
		It("sets transformation config for routes", func() {
			out := &envoyroute.Route{}
			err := p.ProcessRoute(plugins.RouteParams{}, &v1.Route{
				Options: &v1.RouteOptions{
					Transformations: inputTransform,
				},
			}, out)
			Expect(err).NotTo(HaveOccurred())
			Expect(out.TypedPerFilterConfig).To(HaveKeyWithValue(FilterName, expected))
		})
		It("sets filters correctly when no early filters exist", func() {
			filters, err := p.HttpFilters(plugins.Params{}, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(filters)).To(Equal(1))
			value := filters[0].HttpFilter.GetTypedConfig().GetValue()
			Expect(value).To(BeEmpty())
		})
	})

	Context("staged transformations", func() {
		var (
			inputTransform *transformation.TransformationStages
		)
		BeforeEach(func() {
			p = NewPlugin()
			inputTransform = &transformation.TransformationStages{
				Early: &transformation.RequestResponseTransformations{
					RequestTransforms: []*transformation.RequestMatch{
						{
							ClearRouteCache: true,
						},
					},
				},
			}
			outputTransform = &envoytransformation.RouteTransformations{
				// new config should not get deprecated config
				Transformations: []*envoytransformation.RouteTransformations_RouteTransformation{
					{
						Stage: EarlyStageNumber,
						Match: &envoytransformation.RouteTransformations_RouteTransformation_RequestMatch_{
							RequestMatch: &envoytransformation.RouteTransformations_RouteTransformation_RequestMatch{ClearRouteCache: true},
						},
					},
				},
			}
			configStruct, err := utils.MessageToAny(outputTransform)
			Expect(err).NotTo(HaveOccurred())

			expected = configStruct
		})
		It("sets transformation config for routes", func() {
			out := &envoyroute.Route{}
			err := p.ProcessRoute(plugins.RouteParams{}, &v1.Route{
				Options: &v1.RouteOptions{
					StagedTransformations: inputTransform,
				},
			}, out)
			Expect(err).NotTo(HaveOccurred())
			Expect(out.TypedPerFilterConfig).To(HaveKeyWithValue(FilterName, expected))
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
	})

})
