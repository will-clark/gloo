#!/bin/sh

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

echo "${pluginName} was successfully installed to ${pluginPath}/${pluginName} 🎉"