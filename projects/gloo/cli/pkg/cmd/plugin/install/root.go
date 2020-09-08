package install

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

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

					return installPlugin(plugin.Name, desiredVersionedPluginUrl, downloadPath)
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

func installPlugin(name, url, destination string) error {
	cmd := exec.Command("sh", "-c", installScript, "install-plugin-placeholder-arg", url, name, destination)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

const installScript = `
set -eu


# TODO: Checksum validation, configurable destination path

if [ -z "${1-}" ]; then
     echo A plugin URL must be provided
     exit 1
fi

if [ -z "${2-}" ]; then
     echo A plugin name must be provided
     exit 1
fi

if [ -z "${3-}" ]; then
     echo An installation path must be provided
     exit 1
fi

pluginUrl=$1
pluginName=$2
pluginPath=$3

tmp=$(mktemp -d /tmp/gloo-plugin.XXXXXX)

#if curl -f ${pluginUrl} >/dev/null 2>&1; then
#  echo "line 25"
#  echo "Attempting to download ${pluginName} at ${pluginUrl}"
#else
#  echo "${pluginName} not found at ${pluginUrl}"
#  exit 1
#fi

(
  cd "$tmp"

  echo "Downloading ${pluginName}..."

#  SHA=$(curl -sL "${pluginUrl}.sha256" | cut -d' ' -f1)
  curl -L -o "${pluginName}" "${pluginUrl}"
  echo "Download complete!, validating checksum..."
  # TODO restore
#  checksum=$(openssl dgst -sha256 "${pluginName}" | awk '{ print $2 }')
#  if [ "$checksum" != "$SHA" ]; then
#    echo "Checksum validation failed." >&2
#    exit 1
#  fi
#  echo "Checksum valid."
)

(
  cd "$HOME"
  mkdir -p "${pluginPath}"
  mv "${tmp}/${pluginName}" "${pluginPath}/${pluginName}"
  chmod +x "${pluginPath}/${pluginName}"
)

rm -r "$tmp"

echo "${pluginName} was successfully installed to ${pluginPath}/${pluginName} 🎉"`
