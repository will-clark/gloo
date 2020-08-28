package common

import (
	"context"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"github.com/rotisserie/eris"
	"github.com/solo-io/gloo/projects/gloo/cli/pkg/constants"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

// VersionedPlugins is a map from version to map from binary name to download url.
type VersionedPlugins map[string]map[string]string

// RegistryPlugin is a plugin made available via plugin registry.
type RegistryPlugin struct {
	// Name is the name of the plugin, e.g. "glooctl-fed"
	Name string
	// Name is the user-friendly name of the plugin, e.g. "fed"
	DisplayName       string
	AvailableVersions VersionedPlugins
}

// Registry represents a GcsRegsitry of glooctl plugins.
type Registry interface {
	// Search returns all RegistryPlugins in the Registry with names containing the provided query string.
	Search(ctx context.Context, query string) ([]RegistryPlugin, error)
}

// GcsRegistry is a Registry implementation backed by a Google Cloud Storage bucket.
type GcsRegsitry struct {
	Client       *storage.Client
	Bucket       string
	PluginPrefix string
}

// NewGcsRegistry returns a
func NewGcsRegistry(ctx context.Context, bucket, pluginPrefix string) (*GcsRegsitry, error) {
	client, err := storage.NewClient(ctx, option.WithScopes(storage.ScopeReadOnly), option.WithoutAuthentication())
	if err != nil {
		return nil, err
	}

	return &GcsRegsitry{
		Client:       client,
		Bucket:       bucket,
		PluginPrefix: pluginPrefix,
	}, nil
}

func (r *GcsRegsitry) Search(ctx context.Context, query string) ([]RegistryPlugin, error) {
	objects, err := r.listAllObjects(ctx)
	if err != nil {
		return nil, err
	}

	foundPlugins := make(map[string]VersionedPlugins)

	var plugins []RegistryPlugin
	for _, object := range objects {
		// Valid plugin objects are structured as "glooctl-{name}/{semver version}/glooctl-{name}-os-arch{optional .exe}"
		parts := strings.Split(strings.TrimSuffix(object.Name, "/"), "/")
		if len(parts) != 3 {
			continue
		}

		pluginName, version, binaryName := parts[0], parts[1], parts[2]

		if !strings.Contains(pluginName, query) {
			continue
		}

		if _, ok := foundPlugins[pluginName]; !ok {
			foundPlugins[pluginName] = make(map[string]map[string]string)
		}
		if _, ok := foundPlugins[pluginName][version]; !ok {
			foundPlugins[pluginName][version] = make(map[string]string)
		}
		foundPlugins[pluginName][version][binaryName] = object.MediaLink
	}

	for pluginName, versionedPlugins := range foundPlugins {
		plugins = append(plugins, RegistryPlugin{
			Name:              pluginName,
			DisplayName:       strings.TrimPrefix(pluginName, r.PluginPrefix),
			AvailableVersions: versionedPlugins,
		})
	}

	return plugins, nil
}

func (r *GcsRegsitry) listAllObjects(ctx context.Context) ([]*storage.ObjectAttrs, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	var objects []*storage.ObjectAttrs
	it := r.Client.Bucket(constants.GlooctlPluginBucket).Objects(ctx, nil)
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, eris.Wrap(err, "Error listing available plugins")
		}

		objects = append(objects, attrs)
	}
	return objects, nil
}
