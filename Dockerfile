FROM ubuntu:16.04
MAINTAINER openebs <openebs@gmail.com>
RUN apt-get update && apt-get install -y \
             git \
	     automake \
	     gcc \
	     curl \
	     make \
	     apt-transport-https \
	     ca-certificates \
	     && rm -rf /var/lib/apt/lists/*

ENV GOLANG_VERSION 1.7.3
ENV GOLANG_DOWNLOAD_URL https://golang.org/dl/go$GOLANG_VERSION.linux-amd64.tar.gz
ENV GOLANG_DOWNLOAD_SHA256 508028aac0654e993564b6e2014bf2d4a9751e3b286661b0b0040046cf18028e

RUN curl -fsSL "$GOLANG_DOWNLOAD_URL" -o golang.tar.gz \
	&& echo "$GOLANG_DOWNLOAD_SHA256  golang.tar.gz" | sha256sum -c - \
	&& tar -xzf golang.tar.gz -C /usr/local \
        && rm golang.tar.gz

RUN mkdir -p /go
ENV GOPATH=/go
ENV PATH $PATH:/usr/local/go/bin/:$GOPATH/bin
 

# Go tools
RUN go get github.com/rancher/trash
RUN go get github.com/golang/lint/golint

# Docker
RUN curl -sL https://get.docker.com/builds/Linux/x86_64/docker-1.9.1 > /usr/bin/docker && \
chmod +x /usr/bin/docker


RUN mkdir -p $GOPATH/src/github.com/openebs/
RUN cd $GOPATH/src/github.com/openebs/ && \
    git clone https://github.com/openebs/longhorn.git && \
    cd $GOPATH/src/github.com/openebs/longhorn && \
    trash . && \
    make
