# Copyright © 2020 The OpenEBS Authors
#
# This file was originally authored by Rancher Labs
# under Apache License 2018.
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

FROM golang:1.14.7 as build

ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT=""

ENV GO111MODULE=on \
  GOOS=${TARGETOS} \
  GOARCH=${TARGETARCH} \
  GOARM=${TARGETVARIANT} \
  DEBIAN_FRONTEND=noninteractive \
  PATH="/root/go/bin:${PATH}"

WORKDIR /go/src/github.com/openebs/jiva

RUN apt-get update && apt-get install -y make git

COPY go.mod go.sum ./
# Get dependancies - will also be cached if we won't change mod/sum
RUN go mod download

COPY . .

RUN ./scripts/build_binaries

FROM ubuntu:xenial-20200212 

RUN apt-get update && apt-get install -y curl \
    && rm -rf /var/lib/apt/lists/*

ARG DBUILD_DATE
ARG DBUILD_REPO_URL
ARG DBUILD_SITE_URL

LABEL org.label-schema.schema-version="1.0"
LABEL org.label-schema.name="jiva"
LABEL org.label-schema.description="OpenEBS Jiva"
LABEL org.label-schema.build-date=$DBUILD_DATE
LABEL org.label-schema.vcs-url=$DBUILD_REPO_URL
LABEL org.label-schema.url=$DBUILD_SITE_URL

COPY --from=build /go/src/github.com/openebs/jiva/bin/longhorn /usr/local/bin/
COPY --from=build /go/src/github.com/openebs/jiva/bin/longhorn /usr/local/bin/jivactl
COPY --from=build /go/src/github.com/openebs/jiva/package/launch /usr/local/bin/
COPY --from=build /go/src/github.com/openebs/jiva/package/copy-binary /usr/local/bin/
COPY --from=build /go/src/github.com/openebs/jiva/package/launch-with-vm-backing-file /usr/local/bin/
COPY --from=build /go/src/github.com/openebs/jiva/package/launch-simple-jiva /usr/local/bin/

VOLUME /usr/local/bin
CMD ["longhorn"]
