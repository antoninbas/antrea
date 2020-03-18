#!/usr/bin/env bash

set -eo pipefail

THIS_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

OVS_VERSION="2.13.0"
QEMU_TAG="a7996909642ee92942dcd6cff44b9b95f08dad64"
PLATFORM=$1

wget https://github.com/docker/buildx/releases/download/v0.2.0/buildx-v0.2.0.linux-arm64
mkdir -p ~/.docker/cli-plugins/
mv buildx-v0.2.0.linux-arm64 ~/.docker/cli-plugins/docker-buildx
chmod +x ~/.docker/cli-plugins/docker-buildx
docker run --rm --privileged docker/binfmt:$QEMU_TAG
cat /proc/sys/fs/binfmt_misc/qemu-aarch64

docker buildx create --name mybuilder
docker buildx use mybuilder
docker buildx inspect --bootstrap

pushd $THIS_DIR/../build/images/ovs
docker buildx build -t antrea/openvswitch:$OVS_VERSION --platform $PLATFORM --build-arg OVS_VERSION=$OVS_VERSION .
popd

pushd $THIS_DIR/..
docker buildx build -t antoninbas/antrea-ubuntu-march --platform $PLATFORM -f build/images/Dockerfile.build.ubuntu . --push
popd
