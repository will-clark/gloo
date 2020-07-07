package e2e_test

import (
	"context"

	gatewaydefaults "github.com/solo-io/gloo/projects/gateway/pkg/defaults"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/solo-io/gloo/test/services"
	"github.com/solo-io/solo-kit/pkg/api/v1/clients"

	gloov1 "github.com/solo-io/gloo/projects/gloo/pkg/api/v1"
	"github.com/solo-io/gloo/projects/gloo/pkg/api/v1/options/transformation"
	"github.com/solo-io/gloo/projects/gloo/pkg/defaults"
	"github.com/solo-io/gloo/test/v1helpers"
	"github.com/solo-io/solo-kit/pkg/api/v1/resources/core"
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

	setProxy := func(et *transformation.EarlyTransformations) {
		proxy := getTrivialProxyForUpstream(defaults.GlooSystem, envoyPort, up.Metadata.Ref())
		proxy.Listeners[0].ListenerType.(*gloov1.Listener_HttpListener).HttpListener.
			VirtualHosts[0].Options = &gloov1.VirtualHostOptions{
			EarlyTransformations: et,
		}
		_, err := testClients.ProxyClient.Write(proxy, clients.WriteOpts{})
		Expect(err).NotTo(HaveOccurred())
		TestUpstreamReachable()
	}

	It("should should transform response", func() {
		setProxy(&transformation.EarlyTransformations{})
		// send a request an expect it transformed!

	})
})

func getTrivialProxyForUpstream2(ns string, bindPort uint32, upstream core.ResourceRef) *gloov1.Proxy {
	proxy := getTrivialProxy(ns, bindPort)
	proxy.Listeners[0].ListenerType.(*gloov1.Listener_HttpListener).HttpListener.
		VirtualHosts[0].Routes[0].Action.(*gloov1.Route_RouteAction).RouteAction.
		Destination.(*gloov1.RouteAction_Single).Single.DestinationType =
		&gloov1.Destination_Upstream{Upstream: &upstream}
	return proxy
}

func getTrivialProxyForService2(ns string, bindPort uint32, service core.ResourceRef, svcPort uint32) *gloov1.Proxy {
	proxy := getTrivialProxy(ns, bindPort)
	proxy.Listeners[0].ListenerType.(*gloov1.Listener_HttpListener).HttpListener.
		VirtualHosts[0].Routes[0].Action.(*gloov1.Route_RouteAction).RouteAction.
		Destination.(*gloov1.RouteAction_Single).Single.DestinationType =
		&gloov1.Destination_Kube{
			Kube: &gloov1.KubernetesServiceDestination{
				Ref:  service,
				Port: svcPort,
			},
		}
	return proxy
}

func getTrivialProxy2(ns string, bindPort uint32) *gloov1.Proxy {
	return &gloov1.Proxy{
		Metadata: core.Metadata{
			Name:      gatewaydefaults.GatewayProxyName,
			Namespace: ns,
		},
		Listeners: []*gloov1.Listener{{
			Name:        "listener",
			BindAddress: "::",
			BindPort:    bindPort,
			ListenerType: &gloov1.Listener_HttpListener{
				HttpListener: &gloov1.HttpListener{
					VirtualHosts: []*gloov1.VirtualHost{{
						Name:    "virt1",
						Domains: []string{"*"},
						Routes: []*gloov1.Route{{
							Action: &gloov1.Route_RouteAction{
								RouteAction: &gloov1.RouteAction{
									Destination: &gloov1.RouteAction_Single{
										Single: &gloov1.Destination{},
									},
								},
							},
						}},
					}},
				},
			},
		}},
	}
}
