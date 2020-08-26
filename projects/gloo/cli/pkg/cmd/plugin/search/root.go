package search

import (
	"fmt"

	"github.com/solo-io/gloo/projects/gloo/cli/pkg/cmd/options"
	"github.com/solo-io/gloo/projects/gloo/cli/pkg/cmd/plugin/common"
	"github.com/solo-io/gloo/projects/gloo/cli/pkg/constants"
	"github.com/solo-io/gloo/projects/gloo/cli/pkg/flagutils"
	"github.com/spf13/cobra"
)

func RootCmd(opts *options.Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   constants.PLUGIN_SEARCH_COMMAND.Use,
		Short: constants.PLUGIN_SEARCH_COMMAND.Short,
		RunE: func(cmd *cobra.Command, args []string) error {

			var searchTerm string
			if len(args) > 0 {
				searchTerm = args[0]
			}

			registry, err := common.NewGcsRegistry(opts.Top.Ctx, constants.GlooctlPluginBucket, "glooctl-")
			if err != nil {
				return err
			}

			plugins, err := registry.Search(opts.Top.Ctx, searchTerm)
			if err != nil {
				return err
			}

			for _, plugin := range plugins {
				fmt.Println(plugin.Name)
			}

			return nil
		},
	}
	flagutils.AddClusterFlags(cmd.PersistentFlags(), &opts.Cluster)
	return cmd
}
