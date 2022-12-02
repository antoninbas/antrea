# Copyright 2022 Antrea Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

FROM ubuntu:20.04

LABEL maintainer="Antrea <projectantrea-dev@googlegroups.com>"
LABEL description="A Docker image based on Ubuntu 20.04 which includes ethtool, ip tools and iptables."

# See https://github.com/kubernetes-sigs/iptables-wrappers
# /iptables-wrapper-installer.sh will have permissions of 600.
# --chmod=700 doesn't work with older versions of Docker and requires DOCKER_BUILDKIT=1, so we use
# chmod in the RUN command below instead.
ADD https://raw.githubusercontent.com/kubernetes-sigs/iptables-wrappers/9e6ce59c864623ea71a6f7d59c35fcb13a919b87/iptables-wrapper-installer.sh /iptables-wrapper-installer.sh

RUN apt-get update && \
    apt-get install -y --no-install-recommends ethtool iproute2 iptables && \
    rm -rf /var/cache/apt/* /var/lib/apt/lists/* && \
    chmod +x /iptables-wrapper-installer.sh && \
    /iptables-wrapper-installer.sh
