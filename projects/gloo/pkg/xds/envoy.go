package xds

import (
	"fmt"

	core "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/service/cluster/v3"
	endpoint_v3 "github.com/envoyproxy/go-control-plane/envoy/service/endpoint/v3"
	listener_v3 "github.com/envoyproxy/go-control-plane/envoy/service/listener/v3"
	route_v3 "github.com/envoyproxy/go-control-plane/envoy/service/route/v3"
	cache_v3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	server_v3 "github.com/envoyproxy/go-control-plane/pkg/server/v3"

	v1 "github.com/solo-io/gloo/projects/gloo/pkg/api/v1"
	"github.com/solo-io/gloo/projects/gloo/pkg/defaults"
	"google.golang.org/grpc"
)

// Returns the node.metadata.role from the envoy bootstrap config
// if not found, it returns a key for the Fallback snapshot
// which alerts the user their Envoy is missing the required role key.
type ProxyKeyHasherV2 struct{}

func NewNodeHasherV2() *ProxyKeyHasherV2 {
	return &ProxyKeyHasherV2{}
}

func (h *ProxyKeyHasherV2) ID(node *core.Node) string {
	if node.Metadata != nil {
		roleValue := node.Metadata.Fields["role"]
		if roleValue != nil {
			return roleValue.GetStringValue()
		}
	}
	// TODO: use FallbackNodeKey here
	return ""
}

// used to let nodes know they have a bad config
// we assign a "fix me" snapshot for bad nodes
const FallbackNodeKey = "misconfigured-node"

// TODO(ilackarms): expose these as a configuration option (maybe)
var fallbackBindPort = defaults.HttpPort

const (
	fallbackBindAddr   = "::"
	fallbackStatusCode = 500
)

// SnapshotKey of Proxy == Role in Envoy Configmap == "Node" in Envoy semantics
func SnapshotKey(proxy *v1.Proxy) string {
	namespace, name := proxy.GetMetadata().Ref().Strings()
	return fmt.Sprintf("%v~%v", namespace, name)
}

// Called in Syncer when a new set of proxies arrive
// used to trim snapshots whose proxies have been deleted
func GetValidKeys(proxies v1.ProxyList, extensionKeys map[string]struct{}) []string {
	var validKeys []string
	// Get keys from proxies
	for _, proxy := range proxies {
		// This is where we correlate Node ID with proxy namespace~name
		validKeys = append(validKeys, SnapshotKey(proxy))
	}
	for key := range extensionKeys {
		validKeys = append(validKeys, key)
	}
	return validKeys
}

// Returns the node.metadata.role from the envoy bootstrap config
// if not found, it returns a key for the Fallback snapshot
// which alerts the user their Envoy is missing the required role key.
type ProxyKeyHasherV3 struct{}

func NewNodeHasherV3() *ProxyKeyHasherV3 {
	return &ProxyKeyHasherV3{}
}

func (h *ProxyKeyHasherV3) ID(node *core_v3.Node) string {
	if node.Metadata != nil {
		roleValue := node.Metadata.Fields["role"]
		if roleValue != nil {
			return roleValue.GetStringValue()
		}
	}
	// TODO: use FallbackNodeKey here
	return ""
}

// register xDS methods with GRPC server
func SetupEnvoyXdsV3(grpcServer *grpc.Server, envoyServer server_v3.Server, envoyCache cache_v3.SnapshotCache) {
	// check if we need to register
	if _, ok := grpcServer.GetServiceInfo()["envoy.service.endpoint.v3.EndpointDiscoveryService"]; ok {
		return
	}

	cluster_v3.RegisterClusterDiscoveryServiceServer(grpcServer, envoyServer)
	endpoint_v3.RegisterEndpointDiscoveryServiceServer(grpcServer, envoyServer)
	listener_v3.RegisterListenerDiscoveryServiceServer(grpcServer, envoyServer)
	route_v3.RegisterRouteDiscoveryServiceServer(grpcServer, envoyServer)
	_ = envoyCache.SetSnapshot(FallbackNodeKey, fallbackSnapshotV3(fallbackBindAddr, fallbackBindPort, fallbackStatusCode))
}
