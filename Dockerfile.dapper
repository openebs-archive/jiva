FROM openebs/jiva:base-xenial-20180417
# FROM ubuntu:16.04
# FROM arm=armhf/ubuntu:16.04

ARG DAPPER_HOST_ARCH=amd64
ENV HOST_ARCH=${DAPPER_HOST_ARCH} ARCH=${DAPPER_HOST_ARCH}

# Setup environment
ENV PATH /go/bin:$PATH
ENV DAPPER_DOCKER_SOCKET true
ENV DAPPER_ENV TAG REPO
ENV DAPPER_OUTPUT bin
ENV DAPPER_RUN_ARGS --privileged --tmpfs /go/src/github.com/openebs/jiva/integration/.venv:exec --tmpfs /go/src/github.com/openebs/jiva/integration/.tox:exec -v /dev:/host/dev -v /proc:/host/proc
ENV DAPPER_SOURCE /go/src/github.com/openebs/jiva
ENV TRASH_CACHE ${DAPPER_SOURCE}/.trash-cache
WORKDIR ${DAPPER_SOURCE}

# Install packages
RUN apt-get update -o Acquire::CompressionTypes::Order::=gz && apt-get update && \
    apt-get install -y cmake wget curl git less file \
        libglib2.0-dev libkmod-dev libnl-genl-3-dev linux-libc-dev pkg-config psmisc python-tox qemu-utils fuse python-dev \
        devscripts debhelper bash-completion librdmacm-dev libibverbs-dev xsltproc docbook-xsl \
        libconfig-general-perl libaio-dev libc6-dev iptables libltdl7

# needed for ${!var} substitution
RUN rm -f /bin/sh && ln -s /bin/bash /bin/sh

# Install Go & tools
ENV GOLANG_ARCH_amd64=amd64 GOLANG_ARCH_arm=armv6l GOLANG_ARCH=GOLANG_ARCH_${ARCH} \
    GOPATH=/go PATH=/go/bin:/usr/local/go/bin:${PATH} SHELL=/bin/bash
RUN wget -O - https://storage.googleapis.com/golang/go1.10.3.linux-${!GOLANG_ARCH}.tar.gz | tar -xzf - -C /usr/local && \
    go get github.com/rancher/trash && go get -u golang.org/x/lint/golint && go get github.com/prometheus/client_golang/prometheus/promhttp

# Docker
ENV DOCKER_URL_amd64=https://download.docker.com/linux/ubuntu/dists/xenial/pool/stable/amd64/docker-ce_17.03.3~ce-0~ubuntu-xenial_amd64.deb \
DOCKER_URL_arm=https://download.docker.com/linux/ubuntu/dists/xenial/pool/stable/arm64/docker-ce_18.06.1~ce~3-0~ubuntu_arm64.deb \
DOCKER_URL=DOCKER_URL_${ARCH}

RUN wget ${!DOCKER_URL} -O docker_ce_${ARCH} && dpkg -i docker_ce_${ARCH}

VOLUME /tmp
ENV TMPDIR /tmp
ENTRYPOINT ["./scripts/entry"]
CMD ["build"]
