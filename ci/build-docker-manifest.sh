#!/usr/bin/env bash

set -eo pipefail

IMG_NAME=antoninbas/antrea-ubuntu-test

docker manifest create ${IMG_NAME}:latest ${IMG_NAME}:latest-amd64 ${IMG_NAME}:latest-arm64
docker manifest annotate ${IMG_NAME}:latest ${IMG_NAME}:latest-amd64 --os linux --arch amd64
docker manifest annotate ${IMG_NAME}:latest ${IMG_NAME}:latest-arm64 --os linux --arch arm64
docker manifest push ${IMG_NAME}:latest --purge
