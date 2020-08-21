package install

import (
	"github.com/solo-io/gloo/projects/gloo/cli/pkg/cmd/options"
	"github.com/solo-io/gloo/projects/gloo/cli/pkg/constants"
	"github.com/solo-io/go-utils/cliutils"
	"github.com/spf13/cobra"
)

func RootCmd(opts *options.Options, optionsFunc ...cliutils.OptionsFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:     constants.UPGRADE_COMMAND.Use,
		Aliases: constants.UPGRADE_COMMAND.Aliases,
		Short:   constants.UPGRADE_COMMAND.Short,
		RunE: func(cmd *cobra.Command, args []string) error {
			return installPlugin(opts.Top.Ctx, opts.Plugin.Install)
		},
	}

	cmd.PersistentFlags().StringVar(&opts.Plugin.Install.Name, "name", "latest", "Which plugin "+
		"to download. Specify the name of a plugin corresponding to a ")
	cmd.PersistentFlags().StringVar(&opts.Plugin.Install.ReleaseTag, "release", "latest", "Which release "+
		"to download. Specify a git tag corresponding to the desired version of glooctl.")
	cmd.PersistentFlags().StringVar(&opts.Plugin.Install.DownloadPath, "path", "", "Desired path for your "+
		"upgraded glooctl binary. Defaults to the location of your currently executing binary.")
	cliutils.ApplyOptions(cmd, optionsFunc)
	return cmd
}
