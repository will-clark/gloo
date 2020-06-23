package syncer

import (
	"context"

	cache_v3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/solo-io/gloo/projects/gloo/pkg/api/v1/enterprise/options/ratelimit"
	"github.com/solo-io/gloo/projects/gloo/pkg/syncer/sanitizer"

	"github.com/hashicorp/go-multierror"
	v1 "github.com/solo-io/gloo/projects/gloo/pkg/api/v1"
	"github.com/solo-io/gloo/projects/gloo/pkg/translator"
	envoycache "github.com/solo-io/solo-kit/pkg/api/v1/control-plane/cache"
	"github.com/solo-io/solo-kit/pkg/api/v2/reporter"
)

type translatorSyncer struct {
	translator    translator.Translator
	sanitizer     sanitizer.XdsSanitizer
	skXdsCache    envoycache.SnapshotCache
	envoyXdsCache cache_v3.SnapshotCache

	reporter reporter.Reporter
	// used for debugging purposes only
	latestSnap *v1.ApiSnapshot
	extensions []TranslatorSyncerExtension
	// used to track which envoy node IDs exist without belonging to a proxy
	extensionKeys map[string]struct{}
	settings      *v1.Settings
}

type TranslatorSyncerExtensionParams struct {
	Reporter                 reporter.Reporter
	RateLimitServiceSettings ratelimit.ServiceSettings
}

type TranslatorSyncerExtensionFactory func(context.Context, TranslatorSyncerExtensionParams) (TranslatorSyncerExtension, error)

type TranslatorSyncerExtension interface {
	Sync(ctx context.Context, snap *v1.ApiSnapshot, xdsCache envoycache.SnapshotCache) (string, error)
}

func NewTranslatorSyncer(
	translator translator.Translator,
	skXdsCache envoycache.SnapshotCache,
	envoyXdsCache cache_v3.SnapshotCache,
	sanitizer sanitizer.XdsSanitizer,
	reporter reporter.Reporter,
	devMode bool,
	extensions []TranslatorSyncerExtension,
	settings *v1.Settings,
) v1.ApiSyncer {
	s := &translatorSyncer{
		translator:    translator,
		sanitizer:     sanitizer,
		skXdsCache:    skXdsCache,
		envoyXdsCache: envoyXdsCache,
		reporter:      reporter,
		extensions:    extensions,
		settings:      settings,
	}
	if devMode {
		// TODO(ilackarms): move this somewhere else?
		go func() {
			_ = s.ServeXdsSnapshots()
		}()
	}
	return s
}

func (s *translatorSyncer) Sync(ctx context.Context, snap *v1.ApiSnapshot) error {
	var multiErr *multierror.Error
	err := s.syncEnvoy(ctx, snap)
	if err != nil {
		multiErr = multierror.Append(multiErr, err)
	}
	s.extensionKeys = map[string]struct{}{}
	for _, extension := range s.extensions {
		nodeID, err := extension.Sync(ctx, snap, s.skXdsCache)
		if err != nil {
			multiErr = multierror.Append(multiErr, err)
		}
		s.extensionKeys[nodeID] = struct{}{}
	}
	return multiErr.ErrorOrNil()
}
