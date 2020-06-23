package syncer_test

import (
	"context"

	envoy_config_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	cache_v3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/envoyproxy/go-control-plane/pkg/resource/v2"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/solo-io/gloo/projects/gloo/pkg/api/grpc/validation"
	v1 "github.com/solo-io/gloo/projects/gloo/pkg/api/v1"
	"github.com/solo-io/gloo/projects/gloo/pkg/plugins"
	. "github.com/solo-io/gloo/projects/gloo/pkg/syncer"
	"github.com/solo-io/solo-kit/pkg/api/v1/clients"
	"github.com/solo-io/solo-kit/pkg/api/v1/clients/factory"
	"github.com/solo-io/solo-kit/pkg/api/v1/clients/memory"
	"github.com/solo-io/solo-kit/pkg/api/v1/resources/core"
	"github.com/solo-io/solo-kit/pkg/api/v2/reporter"
	"github.com/solo-io/solo-kit/pkg/errors"
)

var _ = Describe("Translate Proxy", func() {

	var (
		xdsCache    *mockXdsCache
		sanitizer   *mockXdsSanitizer
		syncer      v1.ApiSyncer
		snap        *v1.ApiSnapshot
		settings    *v1.Settings
		proxyClient v1.ProxyClient
		proxyName   = "proxy-name"
		ref         = "syncer-test"
		ns          = "any-ns"
	)

	BeforeEach(func() {
		xdsCache = &mockXdsCache{}
		sanitizer = &mockXdsSanitizer{}

		resourceClientFactory := &factory.MemoryResourceClientFactory{
			Cache: memory.NewInMemoryResourceCache(),
		}

		proxyClient, _ = v1.NewProxyClient(resourceClientFactory)

		upstreamClient, err := resourceClientFactory.NewResourceClient(factory.NewResourceClientParams{ResourceType: &v1.Upstream{}})
		Expect(err).NotTo(HaveOccurred())

		proxy := &v1.Proxy{
			Metadata: core.Metadata{
				Namespace: ns,
				Name:      proxyName,
			},
		}

		settings = &v1.Settings{}

		rep := reporter.NewReporter(ref, proxyClient.BaseClient(), upstreamClient)

		syncer = NewTranslatorSyncer(&mockTranslator{true}, nil, xdsCache, sanitizer, rep, false, nil, settings)
		snap = &v1.ApiSnapshot{
			Proxies: v1.ProxyList{
				proxy,
			},
		}
		_, err = proxyClient.Write(proxy, clients.WriteOpts{})
		Expect(err).NotTo(HaveOccurred())
		err = syncer.Sync(context.Background(), snap)
		Expect(err).NotTo(HaveOccurred())

		proxies, err := proxyClient.List(proxy.GetMetadata().Namespace, clients.ListOpts{})
		Expect(err).NotTo(HaveOccurred())
		Expect(proxies).To(HaveLen(1))
		Expect(proxies[0]).To(BeAssignableToTypeOf(&v1.Proxy{}))
		Expect(proxies[0].Status).To(Equal(core.Status{
			State:      2,
			Reason:     "1 error occurred:\n\t* hi, how ya doin'?\n\n",
			ReportedBy: ref,
		}))

		// NilSnapshot is always consistent, so snapshot will always be set as part of endpoints update
		Expect(xdsCache.called).To(BeTrue())

		// update rv for proxy
		p1, err := proxyClient.Read(proxy.Metadata.Namespace, proxy.Metadata.Name, clients.ReadOpts{})
		Expect(err).NotTo(HaveOccurred())
		snap.Proxies[0] = p1

		syncer = NewTranslatorSyncer(&mockTranslator{false}, nil, xdsCache, sanitizer, rep, false, nil, settings)

		err = syncer.Sync(context.Background(), snap)
		Expect(err).NotTo(HaveOccurred())

	})

	It("writes the reports the translator spits out and calls SetSnapshot on the cache", func() {
		proxies, err := proxyClient.List(ns, clients.ListOpts{})
		Expect(err).NotTo(HaveOccurred())
		Expect(proxies).To(HaveLen(1))
		Expect(proxies[0]).To(BeAssignableToTypeOf(&v1.Proxy{}))
		Expect(proxies[0].Status).To(Equal(core.Status{
			State:      1,
			ReportedBy: ref,
		}))

		Expect(xdsCache.called).To(BeTrue())
	})

	It("updates the cache with the sanitized snapshot", func() {
		sanitizer.snap = &cache_v3.Snapshot{}
		err := syncer.Sync(context.Background(), snap)
		Expect(err).NotTo(HaveOccurred())

		Expect(sanitizer.called).To(BeTrue())
		Expect(xdsCache.setSnap).To(BeEquivalentTo(sanitizer.snap))
	})

	It("uses listeners and routes from the previous snapshot when sanitization fails", func() {
		sanitizer.err = errors.Errorf("we ran out of coffee")

		oldXdsSnap := cache_v3.Snapshot{}
		oldXdsSnap.Resources[types.Listener] = cache_v3.NewResources(
			"old listeners from before the war",
			[]types.Resource{
				&envoy_config_listener_v3.Listener{Name: "name"},
			},
		)

		// return this old snapshot when the syncer asks for it
		xdsCache.getSnap = &oldXdsSnap
		err := syncer.Sync(context.Background(), snap)
		Expect(err).NotTo(HaveOccurred())

		Expect(sanitizer.called).To(BeTrue())
		Expect(xdsCache.called).To(BeTrue())

		oldListeners := oldXdsSnap.GetResources(resource.ListenerType)
		newListeners := xdsCache.setSnap.GetResources(resource.ListenerType)

		Expect(oldListeners).To(Equal(newListeners))

		oldRoutes := oldXdsSnap.GetResources(resource.RouteType)
		newRoutes := xdsCache.setSnap.GetResources(resource.RouteType)

		Expect(oldRoutes).To(Equal(newRoutes))
	})
})

type mockTranslator struct {
	reportErrs bool
}

func (t *mockTranslator) Translate(params plugins.Params, proxy *v1.Proxy) (cache_v3.Snapshot, reporter.ResourceReports, *validation.ProxyReport, error) {
	if t.reportErrs {
		rpts := reporter.ResourceReports{}
		rpts.AddError(proxy, errors.Errorf("hi, how ya doin'?"))
		return cache_v3.Snapshot{}, rpts, &validation.ProxyReport{}, nil
	}
	return cache_v3.Snapshot{}, nil, &validation.ProxyReport{}, nil
}

var _ cache_v3.SnapshotCache = &mockXdsCache{}

type mockXdsCache struct {
	called bool
	// snap that is set
	setSnap *cache_v3.Snapshot
	// snap that is returned
	getSnap *cache_v3.Snapshot
}

func (c *mockXdsCache) SetSnapshot(node string, snapshot cache_v3.Snapshot) error {
	c.called = true
	c.setSnap = &snapshot
	return nil
}

func (c *mockXdsCache) GetSnapshot(node string) (cache_v3.Snapshot, error) {
	if c.getSnap != nil {
		return *c.getSnap, nil
	}
	return cache_v3.Snapshot{}, nil
}

func (c *mockXdsCache) GetStatusInfo(s string) cache_v3.StatusInfo {
	panic("implement me")
}

func (c *mockXdsCache) CreateWatch(request cache_v3.Request) (value chan cache_v3.Response, cancel func()) {
	panic("implement me")
}

func (c *mockXdsCache) Fetch(ctx context.Context, request cache_v3.Request) (*cache_v3.Response, error) {
	panic("implement me")
}

func (c *mockXdsCache) GetStatusKeys() []string {
	return []string{}
}

func (*mockXdsCache) ClearSnapshot(node string) {
	panic("implement me")
}

type mockXdsSanitizer struct {
	called bool
	snap   *cache_v3.Snapshot
	err    error
}

func (s *mockXdsSanitizer) SanitizeSnapshot(ctx context.Context, glooSnapshot *v1.ApiSnapshot, xdsSnapshot cache_v3.Snapshot, reports reporter.ResourceReports) (cache_v3.Snapshot, error) {
	s.called = true
	if s.snap != nil {
		return *s.snap, nil
	}
	if s.err != nil {
		return cache_v3.Snapshot{}, s.err
	}
	return xdsSnapshot, nil
}
