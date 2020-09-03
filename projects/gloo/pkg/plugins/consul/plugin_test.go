package consul

import (
	"net"
	"net/url"

	envoyapi "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	envoyauth "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	"github.com/envoyproxy/go-control-plane/pkg/wellknown"
	"github.com/solo-io/gloo/projects/gloo/pkg/api/v1/options/consul"
	"github.com/solo-io/gloo/projects/gloo/pkg/plugins"
	"github.com/solo-io/gloo/projects/gloo/pkg/utils"
	"github.com/solo-io/solo-kit/pkg/api/v1/resources/core"

	v1 "github.com/solo-io/gloo/projects/gloo/pkg/api/v1"

	mock_consul2 "github.com/solo-io/gloo/projects/gloo/pkg/plugins/consul/mocks"

	envoycore "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	"github.com/golang/mock/gomock"
	consulapi "github.com/hashicorp/consul/api"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	mock_consul "github.com/solo-io/gloo/projects/gloo/pkg/upstreams/consul/mocks"
)

var _ = Describe("Resolve", func() {
	var (
		ctrl              *gomock.Controller
		consulWatcherMock *mock_consul.MockConsulWatcher
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(T)

		consulWatcherMock = mock_consul.NewMockConsulWatcher(ctrl)

	})

	AfterEach(func() {
		ctrl.Finish()
	})

	It("can resolve consul service addresses that are IPs", func() {
		plug := NewPlugin(consulWatcherMock, nil, nil)

		svcName := "my-svc"
		tag := "tag"
		dc := "dc1"

		us := createTestFilteredUpstream(svcName, svcName, nil, []string{tag}, []string{dc})

		queryOpts := &consulapi.QueryOptions{Datacenter: dc, RequireConsistent: true}

		consulWatcherMock.EXPECT().Service(svcName, "", queryOpts).Return([]*consulapi.CatalogService{
			{
				ServiceAddress: "5.6.7.8",
				ServicePort:    1234,
			},
			{
				ServiceAddress: "1.2.3.4",
				ServicePort:    1234,
				ServiceTags:    []string{tag},
			},
		}, nil, nil)

		u, err := plug.Resolve(us)
		Expect(err).NotTo(HaveOccurred())

		Expect(u).To(Equal(&url.URL{Scheme: "http", Host: "1.2.3.4:1234"}))
	})

	It("can resolve consul service addresses that are hostnames", func() {

		ips := []net.IPAddr{
			{IP: net.IPv4(2, 1, 0, 10)}, // we will arbitrarily default to the first DNS response
			{IP: net.IPv4(2, 1, 0, 11)},
		}
		mockDnsResolver := mock_consul2.NewMockDnsResolver(ctrl)
		mockDnsResolver.EXPECT().Resolve(gomock.Any(), "test.service.consul").Return(ips, nil).Times(1)

		plug := NewPlugin(consulWatcherMock, mockDnsResolver, nil)

		svcName := "my-svc"
		tag := "tag"
		dc := "dc1"

		us := createTestFilteredUpstream(svcName, svcName, nil, []string{tag}, []string{dc})

		queryOpts := &consulapi.QueryOptions{Datacenter: dc, RequireConsistent: true}

		consulWatcherMock.EXPECT().Service(svcName, "", queryOpts).Return([]*consulapi.CatalogService{
			{
				ServiceAddress: "5.6.7.8",
				ServicePort:    1234,
			},
			{
				ServiceAddress: "test.service.consul",
				ServicePort:    1234,
				ServiceTags:    []string{tag},
			},
		}, nil, nil)

		u, err := plug.Resolve(us)
		Expect(err).NotTo(HaveOccurred())

		Expect(u).To(Equal(&url.URL{Scheme: "http", Host: "2.1.0.10:1234"}))
	})

	It("can resolve consul service addresses in an unfiltered upstream", func() {

		plug := NewPlugin(consulWatcherMock, nil, nil)

		svcName := "my-svc"
		dc := "dc1"

		us := createTestFilteredUpstream(svcName, svcName, nil, nil, []string{dc})

		queryOpts := &consulapi.QueryOptions{Datacenter: dc, RequireConsistent: true}

		consulWatcherMock.EXPECT().Service(svcName, "", queryOpts).Return([]*consulapi.CatalogService{
			{
				ServiceAddress: "5.6.7.8",
				ServicePort:    1234,
			},
		}, nil, nil)

		u, err := plug.Resolve(us)
		Expect(err).NotTo(HaveOccurred())

		Expect(u).To(Equal(&url.URL{Scheme: "http", Host: "5.6.7.8:1234"}))
	})

	It("ProcessUpstream reacts to useTLS flag by adding socket to envoyAPi", func() {
		params := plugins.Params{}
		upstream := &v1.Upstream{
			Metadata: core.Metadata{
				Name:      "upstreamName",
				Namespace: "ns",
			},
			UpstreamType: &v1.Upstream_Consul{
				Consul: &consul.UpstreamSpec{
					UseTls: true,
				},
			},
		}
		out := &envoyapi.Cluster{}

		tlsContext := &envoyauth.UpstreamTlsContext{}
		expectedSocket := &envoycore.TransportSocket{
			Name:       wellknown.TransportSocketTls,
			ConfigType: &envoycore.TransportSocket_TypedConfig{TypedConfig: utils.MustMessageToAny(tlsContext)},
		}

		err := NewPlugin(consulWatcherMock, nil, nil).ProcessUpstream(params, upstream, out)

		Expect(err).NotTo(HaveOccurred())
		Expect(out.TransportSocket).To(Equal(expectedSocket))
	})
})
