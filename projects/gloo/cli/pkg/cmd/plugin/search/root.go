package search

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rotisserie/eris"
	"github.com/solo-io/gloo/projects/gloo/cli/pkg/cmd/options"
	"github.com/solo-io/gloo/projects/gloo/cli/pkg/constants"
	"github.com/solo-io/gloo/projects/gloo/cli/pkg/flagutils"
	"github.com/spf13/cobra"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"k8s.io/kubernetes/pkg/util/slice"

	"cloud.google.com/go/storage"
)

func RootCmd(opts *options.Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   constants.PLUGIN_SEARCH_COMMAND.Use,
		Short: constants.PLUGIN_SEARCH_COMMAND.Short,
		RunE: func(cmd *cobra.Command, args []string) error {
			return listAll()
		},
	}
	flagutils.AddClusterFlags(cmd.PersistentFlags(), &opts.Cluster)
	return cmd
}

func listAll() error {
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
