#!/usr/bin/env bash

set -eo pipefail

THIS_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

OVS_VERSION="2.13.0"

pushd $THIS_DIR/../build/images/ovs
docker build -t antrea/openvswitch:$OVS_VERSION --build-arg OVS_VERSION=$OVS_VERSION .
popd

pushd $THIS_DIR/..
make
docker tag antrea/antrea-ubuntu antoninbas/antrea-ubuntu-test:latest-$1
popd

docker push antoninbas/antrea-ubuntu-test:latest-$1
