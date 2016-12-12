# Makefile for building jiva docker image
# 
# Reference Guide - https://www.gnu.org/software/make/manual/make.html


#
# Internal variables or constants.
# NOTE - These will be executed when any make target is invoked.
#
IS_GO_INSTALLED           := $(shell which go >> /dev/null 2>&1; echo $$?)
IS_DOCKER_INSTALLED       := $(shell which docker >> /dev/null 2>&1; echo $$?)

help:
	@echo ""
	@echo "Usage:-"
	@echo "\tmake build             -- will create jiva image"
	@echo "\tmake deps              -- will verify build dependencies are installed"
	@echo ""


_build_check_go:
	@if [ $(IS_GO_INSTALLED) -eq 1 ]; \
		then echo "" \
		&& echo "ERROR:\tgo is not installed. Please install it before build." \
		&& echo "" \
		&& exit 1; \
		fi;


_build_check_docker:
	@if [ $(IS_DOCKER_INSTALLED) -eq 1 ]; \
		then echo "" \
		&& echo "ERROR:\tdocker is not installed. Please install it before build." \
		&& echo "" \
		&& exit 1; \
		fi;

deps: _build_check_go _build_check_docker
	@echo ""
	@echo "INFO:\tverifying dependencies for jiva ..."

_install_trash:
	go get -u github.com/rancher/trash

_fetch_longhorn:
	mkdir -p $(GOPATH)/src/github.com/openebs
	@if [ ! -d $(GOPATH)/src/github.com/openebs/longhorn ]; \
		then \
	          cd $(GOPATH)/src/github.com/openebs && git clone https://github.com/openebs/longhorn.git; \
		fi;
	cd $(GOPATH)/src/github.com/openebs/longhorn && git pull && trash .

_customize_longhorn:
	cp -R $(GOPATH)/src/github.com/openebs/jiva/package/* $(GOPATH)/src/github.com/openebs/longhorn/package/
	cp -R $(GOPATH)/src/github.com/openebs/jiva/scripts/* $(GOPATH)/src/github.com/openebs/longhorn/scripts/

_build_longhorn:
	cd $(GOPATH)/src/github.com/openebs/longhorn && make

_push_image:
	$(GOPATH)/src/github.com/openebs/longhorn/scripts/push
        
#
# Will build the go based binaries
# The binaries will be placed at $GOPATH/bin/
#
build: deps _install_trash _fetch_longhorn _customize_longhorn _build_longhorn _push_image
	@echo ""
	@echo "INFO:\t..... verify that jiva image is created"
	@echo "INFO:\t..... run ci over jiva image"
	@echo ""



#
# This is done to avoid conflict with a file of same name as the targets
# mentioned in this makefile.
#
.PHONY: help deps build 
.DEFAULT_GOAL := build

