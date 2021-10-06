#!/usr/bin/env bash

set -Eeuo pipefail
ROOT="$( cd "$( dirname "$0" )" && cd ../.. && pwd -P )"
DST="${ROOT}"/aws-lambda-extension/soraproxy.zip
TMP=$(mktemp -d -t soraproxy)

echo Working at "${TMP}"
mkdir -p "${TMP}"/{extensions,bin}

echo Copying extension
cp aws-lambda-extension/extensions/extension.sh "${TMP}"/extensions

echo Copying soraproxy
cp dist/soraproxy_linux_amd64/soraproxy "${TMP}"/bin

echo Getting curl command
curl -sL https://github.com/moparisthebest/static-curl/releases/download/v7.78.0/curl-amd64 -o "${TMP}"/curl

echo Zipping up files
cd "${TMP}"
zip -r "${DST}" extensions
zip -r "${DST}" bin

echo Cleaning up "${TMP}"
rm -fr "${TMP}"

echo Created "${DST}"
