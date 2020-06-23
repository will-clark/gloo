package bootstrap

import (
	"context"
	"net"
	"time"

	"github.com/solo-io/gloo/projects/gloo/pkg/validation"

	"github.com/solo-io/gloo/projects/gloo/pkg/upstreams/consul"

	cache_v3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	server_v3 "github.com/envoyproxy/go-control-plane/pkg/server/v3"
	v1 "github.com/solo-io/gloo/projects/gloo/pkg/api/v1"
	"github.com/solo-io/solo-kit/pkg/api/v1/clients"
	"github.com/solo-io/solo-kit/pkg/api/v1/clients/factory"
	corecache "github.com/solo-io/solo-kit/pkg/api/v1/clients/kube/cache"
	"github.com/solo-io/solo-kit/pkg/api/v1/control-plane/cache"
	"github.com/solo-io/solo-kit/pkg/api/v1/control-plane/server"
	skkube "github.com/solo-io/solo-kit/pkg/api/v1/resources/common/kubernetes"
	"google.golang.org/grpc"
	"k8s.io/client-go/kubernetes"
)

type Opts struct {
	WriteNamespace    string
	WatchNamespaces   []string
	Upstreams         factory.ResourceClientFactory
	KubeServiceClient skkube.ServiceClient
	UpstreamGroups    factory.ResourceClientFactory
	Proxies           factory.ResourceClientFactory
	Secrets           factory.ResourceClientFactory
	Artifacts         factory.ResourceClientFactory
	AuthConfigs       factory.ResourceClientFactory
	KubeClient        kubernetes.Interface
	Consul            Consul
	WatchOpts         clients.WatchOpts
	DevMode           bool
	ControlPlane      ControlPlane
	ValidationServer  ValidationServer
	Settings          *v1.Settings
	KubeCoreCache     corecache.KubeCoreCache
}

type Consul struct {
	ConsulWatcher      consul.ConsulWatcher
	DnsServer          string
	DnsPollingInterval *time.Duration
}

type ControlPlane struct {
	*GrpcService
	// The generic snapshot cache used by the sk Xds Server. Can handle generic resources such as rate-limiting/extauth
	SkSnapshotCache cache.SnapshotCache
	// solo-kit XDS server, works via ADS V2
	SkXDSServer     server.Server
	// Snapshot cache used by go-control-plane XDS server V3
	SnapshotCacheV3 cache_v3.SnapshotCache
	// go-control-plane XDS server, works via ADS V3
	XDSServerV3     server_v3.Server
}

type ValidationServer struct {
	*GrpcService
	Server validation.ValidationServer
}

type GrpcService struct {
	Ctx             context.Context
	BindAddr        net.Addr
	GrpcServer      *grpc.Server
	StartGrpcServer bool
}
