THIS_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
export CGO_LDFLAGS="-L$THIS_DIR/ddlog/libs -lnetworkpolicy_controller_ddlog"
export CGO_CPPFLAGS="-I$THIS_DIR/ddlog"
export LD_LIBRARY_PATH="$THIS_DIR/ddlog/libs"

for pkg in "github.com/vmware-tanzu/antrea/pkg/controller/ddlog" "github.com/vmware-tanzu/antrea/pkg/controller/networkpolicy"; do
    for t in "TestPerf1" "TestPerf2" "TestPerf3" "TestPerf4" "TestPerf5"; do
        for i in {1..3}; do
            echo "=========== $pkg $t"
            /usr/bin/time -v go test -v -run=$t -count=1 $pkg
        done
    done
done
