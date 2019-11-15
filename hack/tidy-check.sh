#!/usr/bin/env bash
# Copyright 2019 Antrea Authors
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
set +e
THIS_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
pushd "$THIS_DIR" > /dev/null || exit
MOD_FILE="$THIS_DIR/../go.mod"
SUM_FILE="$THIS_DIR/../go.sum"

general_help() {
	echo 'Please run the following command to generate a new go.mod & go.sum:
docker run -w /antrea -v `pwd`:/antrea golang:1.12 go mod tidy && sudo chown $(id -u):$(id -g) go.mod go.sum'
}

precheck() {
  if [ ! -r "$MOD_FILE" ]; then
    echo "no go.mod found"
    general_help
    exit 1
  fi
  if [ ! -r "$SUM_FILE" ]; then
    echo "no go.sum found"
    general_help
    exit 1
  fi
}

tidy() {
  cp "$MOD_FILE" .go.mod
  cp "$SUM_FILE" .go.sum
  go mod tidy > /dev/null 2>&1
}

clean() {
  mv .go.mod "$MOD_FILE"
  mv .go.sum "$SUM_FILE"
}

failed() {
  echo "'go mod tidy' failed, there are errors in dependencies rules"
  general_help
  clean
  exit 1
}

check() {
  MOD_DIFF=$(diff "$MOD_FILE" .go.mod)
  SUM_DIFF=$(diff "$SUM_FILE" .go.sum)
  if [ -n "$MOD_DIFF" ] || [ -n "$SUM_DIFF" ]; then
    echo "dependencies are not tidy"
    general_help
    clean
    exit 1
  fi
  clean
}

precheck
if tidy;
  then check
  else failed
fi

popd > /dev/null || exit
