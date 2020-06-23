package sanitizer

import (
	"context"

	cache_v3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	v1 "github.com/solo-io/gloo/projects/gloo/pkg/api/v1"
	"github.com/solo-io/solo-kit/pkg/api/v2/reporter"
)

// an XdsSanitizer modifies an xds snapshot before it is stored in the xds cache
// the if the sanitizer returns an error, Gloo will not update the xds cache with the snapshot
// else Gloo will assume the snapshot is valid to send to Envoy
type XdsSanitizer interface {
	SanitizeSnapshot(ctx context.Context, glooSnapshot *v1.ApiSnapshot, xdsSnapshot cache_v3.Snapshot, reports reporter.ResourceReports) (cache_v3.Snapshot, error)
}

type XdsSanitizers []XdsSanitizer

func (s XdsSanitizers) SanitizeSnapshot(ctx context.Context, glooSnapshot *v1.ApiSnapshot, xdsSnapshot cache_v3.Snapshot, reports reporter.ResourceReports) (cache_v3.Snapshot, error) {
	for _, sanitizer := range s {
		var err error
		xdsSnapshot, err = sanitizer.SanitizeSnapshot(ctx, glooSnapshot, xdsSnapshot, reports)
		if err != nil {
			return cache_v3.Snapshot{}, err
		}
	}
	return xdsSnapshot, nil
}
