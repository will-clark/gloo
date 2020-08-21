package common

import (
	"context"
	"fmt"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"github.com/rotisserie/eris"
	"github.com/solo-io/gloo/projects/gloo/cli/pkg/constants"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"k8s.io/kubernetes/pkg/util/slice"
)

type VersionedPlugin struct {
	Tag         string
	DownloadURL string
}

type RegistryPlugin struct {
	Name              string
	AvailableVersions []VersionedPlugin
}

// Registry represents a gcsRegsitry of glooctl plugins.
type Registry interface {
	// Search returns all RegistryPlugins in the Registry with names containing the provided query string.
	Search(ctx context.Context, query string) ([]RegistryPlugin, error)
}

type gcsRegsitry struct {
	client *storage.Client
	bucket string
}

func NewGcsRegistry(ctx context.Context, bucket string) (Registry, error) {
	client, err := storage.NewClient(ctx, option.WithScopes(storage.ScopeReadOnly), option.WithoutAuthentication())
	if err != nil {
		return nil, err
	}

	return &gcsRegsitry{
		client: client,
		bucket: constants.GlooctlPluginBucket,
	}, nil
}

func (r *gcsRegsitry) Search(ctx context.Context, query string) ([]RegistryPlugin, error) {
	_, err := r.listAllObjects(ctx)
	if err != nil {
		return nil, err
	}

	return nil, nil

	//var plugins []RegistryPlugin
	//for _, object := range objects {
	//
	//}
	//
	//return plugins, nil

	// glooctl-fed/v0.0.19/glooctl-fed-darwin-amd64
}

func (r *gcsRegsitry) listAllObjects(ctx context.Context) ([]*storage.ObjectAttrs, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	var objects []*storage.ObjectAttrs
	it := r.client.Bucket(constants.GlooctlPluginBucket).Objects(ctx, nil)
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

func gcsObjectsToRegistryPlugins(objects *storage.ObjectAttrs) []RegistryPlugin {
	return nil
}

func listAll(searchTerm string) error {
	bucketObjects, err := listAllBucketObjects()
	if err != nil {
		return err
	}

	uniquePlugins := make(map[string]interface{})
	for _, bucketObject := range bucketObjects {
		pluginPathElements := strings.Split(bucketObject, "/")
		if len(pluginPathElements) < 1 {
			continue
		}

		pluginName := pluginPathElements[0]
		uniquePlugins[pluginName] = true
	}

	var pluginList []string
	for plugin := range uniquePlugins {
		if !strings.Contains(plugin, searchTerm) {
			continue
		}

		pluginList = append(pluginList, plugin)
	}

	for _, plugin := range slice.SortStrings(pluginList) {
		fmt.Println(plugin)
	}
	return nil
}

func listAllBucketObjects() ([]string, error) {
	ctx := context.Background()
	client, err := storage.NewClient(ctx, option.WithScopes(storage.ScopeReadOnly), option.WithoutAuthentication())
	if err != nil {
		return nil, err
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	var objects []string
	it := client.Bucket(constants.GlooctlPluginBucket).Objects(ctx, &storage.Query{Prefix: "glooctl-"})
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, eris.Wrap(err, "Error listing available glooctl plugins")
		}
		objects = append(objects, attrs.Name)
	}
	return objects, nil
}
