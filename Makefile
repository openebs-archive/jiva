# Makefile for building jiva docker image
#
# Reference Guide - https://www.gnu.org/software/make/manual/make.html
#

#
# Internal variables or constants.
# NOTE - These will be executed when any make target is invoked.
#

IS_GO_INSTALLED           := $(shell which go >> /dev/null 2>&1; echo $$?)
IS_DOCKER_INSTALLED       := $(shell which docker >> /dev/null 2>&1; echo $$?)

TARGETS := $(shell ls scripts)

# Determine the arch/os
ifeq (${XC_OS}, )
  XC_OS:=$(shell go env GOOS)
endif
export XC_OS

ifeq (${XC_ARCH}, )
  XC_ARCH:=$(shell go env GOARCH)
endif
export XC_ARCH

ARCH:=${XC_OS}_${XC_ARCH}
export ARCH

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


mod:  go.mod go.sum
	@echo "INFO:\tVendor update"
	@GO111MODULE=on go mod download
	@GO111MODULE=on go mod vendor

deps: _build_check_go _build_check_docker mod
	@echo "INFO:\tVerifying dependencies for jiva"


_run_ci:
	@echo "INFO:\tRun ci over jiva image"
	sudo bash ./ci/start_init_test.sh

build_image:
	@echo "INFO:\tRun unit tests and build image"
	bash ./scripts/ci


_push_image:
	cd $(GOPATH)/src/github.com/openebs/jiva && IMAGE_REPO="openebs/jiva" ./scripts/push

#
# Will build the go based binaries
# The binaries will be placed at $GOPATH/bin/
#
# build: deps _install_trash _fetch_longhorn _customize_longhorn _build_longhorn _run_ci _push_image
#
golint := $(shell which golint 2> /dev/null )

lint:
ifndef golint
	$(error "golint is not available in GOPATH. You can install it - go get -u golang.org/x/lint/golint ")
endif
	@echo "Linting with golint"
	$(shell golint $(shell find . -maxdepth 1 -type d \( ! -iname ".git" ! -iname "vendor" \)) )

build: deps build_image _run_ci _push_image
build_gitlab: deps build_image _push_image

#
# This is done to avoid conflict with a file of same name as the targets
# mentioned in this makefile.
#

.PHONY: help deps build $(TARGETS)
.DEFAULT_GOAL := build
