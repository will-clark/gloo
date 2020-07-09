package e2e_test

import (
	"context"

	gatewaydefaults "github.com/solo-io/gloo/projects/gateway/pkg/defaults"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/solo-io/gloo/test/services"
	"github.com/solo-io/solo-kit/pkg/api/v1/clients"

	envoytransformation "github.com/solo-io/gloo/projects/gloo/pkg/api/external/envoy/extensions/transformation"
	gloov1 "github.com/solo-io/gloo/projects/gloo/pkg/api/v1"
	"github.com/solo-io/gloo/projects/gloo/pkg/api/v1/core/matchers"
	"github.com/solo-io/gloo/projects/gloo/pkg/api/v1/options/transformation"
	"github.com/solo-io/gloo/projects/gloo/pkg/defaults"
	"github.com/solo-io/gloo/test/v1helpers"
)

var _ = Describe("Transformation", func() {

	var (
		ctx           context.Context
		cancel        context.CancelFunc
		testClients   services.TestClients
		envoyInstance *services.EnvoyInstance
		tu            *v1helpers.TestUpstream
		envoyPort     uint32
		up            *gloov1.Upstream
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())
		defaults.HttpPort = services.NextBindPort()
		defaults.HttpsPort = services.NextBindPort()

		var err error
		envoyInstance, err = envoyFactory.NewEnvoyInstance()
		Expect(err).NotTo(HaveOccurred())

		tu = v1helpers.NewTestHttpUpstream(ctx, envoyInstance.LocalAddr())
		envoyPort = defaults.HttpPort

		ns := defaults.GlooSystem
		ro := &services.RunOptions{
			NsToWrite: ns,
			NsToWatch: []string{"default", ns},
			WhatToRun: services.What{
				DisableGateway: true,
				DisableUds:     true,
				DisableFds:     true,
			},
		}
		testClients = services.RunGlooGatewayUdsFds(ctx, ro)
		err = envoyInstance.RunWithRole(ns+"~"+gatewaydefaults.GatewayProxyName, testClients.GlooPort)
		Expect(err).NotTo(HaveOccurred())

		up = tu.Upstream
		_, err = testClients.UpstreamClient.Write(up, clients.WriteOpts{})
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		if envoyInstance != nil {
			_ = envoyInstance.Clean()
		}
		cancel()
	})

	TestUpstreamReachable := func() {
		v1helpers.TestUpstreamReachable(envoyPort, tu, nil)
	}

	setProxy := func(et *transformation.TransformationStages) {
		proxy := getTrivialProxyForUpstream(defaults.GlooSystem, envoyPort, up.Metadata.Ref())
		proxy.Listeners[0].ListenerType.(*gloov1.Listener_HttpListener).HttpListener.
			VirtualHosts[0].Options = &gloov1.VirtualHostOptions{
			StagedTransformations: et,
		}
		_, err := testClients.ProxyClient.Write(proxy, clients.WriteOpts{})
		Expect(err).NotTo(HaveOccurred())
		TestUpstreamReachable()
	}

	It("should should transform response", func() {
		setProxy(&transformation.TransformationStages{
			Early: &transformation.RequestResponseTransformations{
				ResponseTransforms: []*transformation.ResponseMatch{{
					Matchers: []*matchers.HeaderMatcher{
						{
							Name:  ":status",
							Value: "200",
						},
					},
					ResponseTransformation: &envoytransformation.Transformation{
						TransformationType: &envoytransformation.Transformation_TransformationTemplate{
							TransformationTemplate: &envoytransformation.TransformationTemplate{
								ParseBodyBehavior: envoytransformation.TransformationTemplate_DontParse,
								BodyTransformation: &envoytransformation.TransformationTemplate_Body{
									Body: &envoytransformation.InjaTemplate{
										Text: "early-transformed",
									},
								},
							},
						},
					},
				}},
			},
			// add regular response to see that the early one overrides it
			Regular: &transformation.RequestResponseTransformations{
				ResponseTransforms: []*transformation.ResponseMatch{{
					Matchers: []*matchers.HeaderMatcher{
						{
							Name:  ":status",
							Value: "200",
						},
					},
					ResponseTransformation: &envoytransformation.Transformation{
						TransformationType: &envoytransformation.Transformation_TransformationTemplate{
							TransformationTemplate: &envoytransformation.TransformationTemplate{
								ParseBodyBehavior: envoytransformation.TransformationTemplate_DontParse,
								BodyTransformation: &envoytransformation.TransformationTemplate_Body{
									Body: &envoytransformation.InjaTemplate{
										Text: "regular-transformed",
									},
								},
							},
						},
					},
				}},
			},
		})
		// send a request an expect it transformed!
		body := []byte("test")
		v1helpers.ExpectHttpOK(body, nil, envoyPort, "early-transformed")
	})
	It("should should transform ext auth", func() {
		setProxy(&transformation.TransformationStages{
			Early: &transformation.RequestResponseTransformations{
				ResponseTransforms: []*transformation.ResponseMatch{{
					ResponseCodeDetails: "foo",
					ResponseTransformation: &envoytransformation.Transformation{
						TransformationType: &envoytransformation.Transformation_TransformationTemplate{
							TransformationTemplate: &envoytransformation.TransformationTemplate{
								ParseBodyBehavior: envoytransformation.TransformationTemplate_DontParse,
								BodyTransformation: &envoytransformation.TransformationTemplate_Body{
									Body: &envoytransformation.InjaTemplate{
										Text: "early-transformed",
									},
								},
							},
						},
					},
				}},
			},
		})
		// send a request an expect it transformed!
		body := []byte("test")
		v1helpers.ExpectHttpOK(body, nil, envoyPort, "early-transformed")
	})
})
