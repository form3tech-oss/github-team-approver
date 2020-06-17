#!/bin/bash

set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"
CHARTS_ROOT="${ROOT}/kubernetes/charts"
TMP_DIR="${ROOT}/tmp/kubernetes/charts"
HELM="${HELM_BIN:-/usr/local/bin/helm}"

rm -rf "${TMP_DIR}"
mkdir -p "${TMP_DIR}"

pushd "${CHARTS_ROOT}" > /dev/null

"${HELM}" template ./github-team-approver \
    --name "github-team-approver" \
    --output-dir "${TMP_DIR}" \
    --set github.app.id="${GITHUB_APP_ID}" \
    --set github.app.installationId="${GITHUB_APP_INSTALLATION_ID}" \
    --set logLevel="debug" \
    --set namespaceOverride="${NAMESPACE}"

popd > /dev/null
