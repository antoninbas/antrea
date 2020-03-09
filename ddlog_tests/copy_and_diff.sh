#!/usr/bin/env bash

set -eo pipefail

function echoerr {
    >&2 echo "$@"
}

KUBECONFIG_OPTION=""
OUT_DIR=""

_usage="Usage: $0 [--kubeconfig <Kubeconfig>] [--out <OutDir>
Retrieved and diff DDlog output files.
        --kubeconfig Kubeconfig            Explicit path to Kubeconfig file. You may also set the KUBECONFIG environment variable.
        --out OutDir                       Where to copy the files to
        --help, -h                         Print this message and exit
"

function print_usage {
    echoerr "$_usage"
}

function print_help {
    echoerr "Try '$0 --help' for more information."
}

while [[ $# -gt 0 ]]
do
key="$1"

case $key in
    --kubeconfig)
    KUBECONFIG_OPTION="--kubeconfig $2"
    shift 2
    ;;
    --out)
    OUT_DIR="$2"
    shift 2
    ;;
    -h|--help)
    print_usage
    exit 0
    ;;
    *)    # unknown option
    echoerr "Unknown option $1"
    exit 1
    ;;
esac
done

if [[ $OUT_DIR == "" ]]; then
    echoerr "--out is required"
    exit 1
fi

CONTROLLER_POD=$(kubectl $KUBECONFIG_OPTION get -n kube-system pods -l component=antrea-controller -oname | cut -c 5-)

kubectl $KUBECONFIG_OPTION cp kube-system/$CONTROLLER_POD:/var/run/antrea-ddlog $OUT_DIR

diff $OUT_DIR/applied-to.txt $OUT_DIR/applied-to.ref.txt
diff $OUT_DIR/address-group.txt $OUT_DIR/address-group.ref.txt
diff $OUT_DIR/np.txt $OUT_DIR/np.ref.txt
