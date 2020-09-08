package install

import (
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strings"

	"github.com/inconshreveable/go-update"
	"github.com/rotisserie/eris"
	"github.com/solo-io/gloo/projects/gloo/cli/pkg/cmd/options"
	"github.com/solo-io/gloo/projects/gloo/cli/pkg/cmd/plugin/common"
	"github.com/solo-io/gloo/projects/gloo/cli/pkg/constants"
	"github.com/solo-io/go-utils/cliutils"
	"github.com/spf13/cobra"
)

func RootCmd(opts *options.Options, optionsFunc ...cliutils.OptionsFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:   constants.PLUGIN_INSTALL_COMMAND.Use,
		Short: constants.PLUGIN_INSTALL_COMMAND.Short,
		RunE: func(cmd *cobra.Command, args []string) error {
			var desiredPlugin string
			if len(args) > 0 {
				desiredPlugin = args[0]
			}

			if desiredPlugin == "" {
				return eris.New("Please specify a plugin to install.")
			}

			registry, err := common.NewGcsRegistry(opts.Top.Ctx, constants.GlooctlPluginBucket, "glooctl-")
			if err != nil {
				return err
			}

			plugins, err := registry.Search(opts.Top.Ctx, desiredPlugin)
			if err != nil {
				return err
			}

			for _, plugin := range plugins {
				if plugin.DisplayName == desiredPlugin {

					// TODO get version
					version := "v0.0.19"

					var fileExtension string
					if runtime.GOOS == "windows" {
						fileExtension = ".exe"
					}
					binaryName := fmt.Sprintf("%s-%s-amd64%s", plugin.Name, runtime.GOOS, fileExtension)

					desiredVersionedPluginUrl := plugin.AvailableVersions[version][binaryName]

					downloadPath := opts.Plugin.Install.DownloadPath
					if downloadPath == "" {
						executablePath, err := os.Executable()
						if err != nil {
							return err
						}
						splitPath := strings.Split(executablePath, string(os.PathSeparator))
						downloadPath = strings.Join(splitPath[0:len(splitPath)-1], string(os.PathSeparator))
					}
					destination := strings.Join([]string{downloadPath, plugin.Name}, string(os.PathSeparator))

					fmt.Printf("url %v file %v", desiredVersionedPluginUrl, destination)

					return nil
				}
			}

			return eris.Errorf("There is no available plugin with name %s", desiredPlugin)
		},
	}

	cmd.PersistentFlags().StringVar(&opts.Plugin.Install.Name, "name", "latest", "Which plugin "+
		"to download. To view available plugins use `glooctl plugin search`")
	cmd.PersistentFlags().StringVar(&opts.Plugin.Install.ReleaseTag, "release", "latest", "Which release "+
		"to download.")
	cmd.PersistentFlags().StringVar(&opts.Plugin.Install.DownloadPath, "path", "", "Desired path for your "+
		"glooctl plugin. Defaults to the location of your glooctl binary.")
	cliutils.ApplyOptions(cmd, optionsFunc)
	return cmd
}

func downloadAsset(downloadUrl string, destFile string) error {
	res, err := http.Get(downloadUrl)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if err := update.Apply(res.Body, update.Options{
		TargetPath: destFile,
	}); err != nil {
		return err
	}

	return nil
}
