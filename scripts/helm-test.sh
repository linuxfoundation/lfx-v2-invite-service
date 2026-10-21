#!/usr/bin/env bash
# Copyright The Linux Foundation and each contributor to LFX.
# SPDX-License-Identifier: MIT
#
# Render tests for the chart's container image reference. Tagged releases
# write image.digest into the packaged chart; the digest must only be used
# when the effective tag equals Chart.appVersion.
#
# Usage: scripts/helm-test.sh [chart-path]

set -euo pipefail

CHART="${1:-charts/lfx-v2-invite-service}"
REPO="ghcr.io/linuxfoundation/lfx-v2-invite-service/invite-api"
DIGEST="sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
OTHER_TAG="0.0.0-other"

APP_VERSION="$(helm show chart "$CHART" | awk '/^appVersion:/ {gsub(/"/, "", $2); print $2}')"
if [[ -z "$APP_VERSION" ]]; then
	echo "FAIL: could not read appVersion from $CHART/Chart.yaml"
	exit 1
fi

failures=0

# render_image prints the rendered container image reference for the given
# helm template arguments.
render_image() {
	helm template test "$CHART" --show-only templates/deployment.yaml "$@" |
		awk '$1 == "image:" {gsub(/"/, "", $2); print $2}'
}

# check asserts that rendering with the given arguments yields the expected image.
check() {
	local name="$1" expected="$2" actual
	shift 2
	actual="$(render_image "$@")"
	if [[ "$actual" == "$expected" ]]; then
		echo "PASS: $name"
	else
		echo "FAIL: $name"
		echo "  expected: $expected"
		echo "  actual:   $actual"
		failures=$((failures + 1))
	fi
}

check "empty digest uses tag" \
	"$REPO:$APP_VERSION"

check "digest with default tag pins by digest" \
	"$REPO:$APP_VERSION@$DIGEST" \
	--set-string image.digest="$DIGEST"

check "digest with tag equal to appVersion pins by digest" \
	"$REPO:$APP_VERSION@$DIGEST" \
	--set-string image.digest="$DIGEST" --set-string image.tag="$APP_VERSION"

check "digest with development tag override uses tag" \
	"$REPO:development" \
	--set-string image.digest="$DIGEST" --set-string image.tag=development

check "digest with different version tag override uses tag" \
	"$REPO:$OTHER_TAG" \
	--set-string image.digest="$DIGEST" --set-string image.tag="$OTHER_TAG"

if ((failures > 0)); then
	echo "$failures chart render test(s) failed"
	exit 1
fi
echo "All chart render tests passed"
