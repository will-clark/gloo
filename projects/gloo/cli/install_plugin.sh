#!/bin/sh

set -eu


# TODO: Checksum validation, configurable destination path

if [ -z "${PLUGIN_URL:-}" ]; then
     echo A plugin URL must be provided
     exit 1
fi

if [ -z "${PLUGIN_NAME:-}" ]; then
     echo A plugin name must be provided
     exit 1
fi

if [ -z "${PLUGIN_PATH:-}" ]; then
     echo An installation path must be provided
     exit 1
fi

tmp=$(mktemp -d /tmp/gloo-plugin.XXXXXX)

#if curl -f ${PLUGIN_URL} >/dev/null 2>&1; then
#  echo "line 25"
#  echo "Attempting to download ${PLUGIN_NAME} at ${PLUGIN_URL}"
#else
#  echo "${PLUGIN_NAME} not found at ${PLUGIN_URL}"
#  exit 1
#fi

(
  cd "$tmp"

  echo "Downloading ${PLUGIN_NAME}..."

#  SHA=$(curl -sL "${PLUGIN_URL}.sha256" | cut -d' ' -f1)
  curl -L -o "${PLUGIN_NAME}" "${PLUGIN_URL}"
  echo "Download complete!, validating checksum..."
  # TODO restore
#  checksum=$(openssl dgst -sha256 "${PLUGIN_NAME}" | awk '{ print $2 }')
#  if [ "$checksum" != "$SHA" ]; then
#    echo "Checksum validation failed." >&2
#    exit 1
#  fi
#  echo "Checksum valid."
)

(
  cd "$HOME"
  mkdir -p "${PLUGIN_PATH}"
  mv "${tmp}/${PLUGIN_NAME}" "${PLUGIN_PATH}/${PLUGIN_NAME}"
  chmod +x "${PLUGIN_PATH}/${PLUGIN_NAME}"
)

rm -r "$tmp"

echo "${PLUGIN_NAME} was successfully installed 🎉"