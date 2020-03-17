#!/usr/bin/env bash

set -eo pipefail

THIS_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

OVS_VERSION="2.13.0"

docker buildx create --name mybuilder
docker buildx use mybuilder
docker buildx inspect --bootstrap

pushd $THIS_DIR/../build/images/ovs
docker buildx build -t antrea/openvswitch:$OVS_VERSION --build-arg OVS_VERSION=$OVS_VERSION .
popd

pushd $THIS_DIR/..
docker buildx build --platform $1 -f build/images/Dockerfile.build.ubuntu .
popd
