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

ifeq (${IMAGE_ORG}, )
  IMAGE_ORG = openebs
  export IMAGE_ORG
endif

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
	@echo "\tARCH = $(ARCH)     -- arch where make is running"
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

_run_ci: test_functional test_features test_resiliency
	@echo "INFO:\tRun ci over jiva image"
	sudo -E bash ./ci/start_init_test.sh

test_features:
	sudo -E bash -x ./ci/feature_tests.sh

test_resiliency:
	sudo -E bash -x ./ci/resiliency_tests.sh

test_functional:
	go build --tags=debug && cp ./jiva tests/functional/
	cd tests/functional && go build --tags=debug && sudo bash -x test.sh

test_e2e:
	cd tests/e2e && go build && sudo ./e2e

build_image:
	@echo "INFO:\tRun unit tests and build image"
	bash ./scripts/ci


_push_image:
	DIMAGE="${IMAGE_ORG}/jiva" ./scripts/push

golint := $(shell which golint 2> /dev/null )

lint:
ifndef golint
	$(error "golint is not available in GOPATH. You can install it - go get -u golang.org/x/lint/golint ")
endif
	@echo "Linting with golint"
	$(shell golint $(shell find . -maxdepth 1 -type d \( ! -iname ".git" ! -iname "vendor" \)) )

build: deps build_image
build_gitlab: deps build_image _push_image

#
# This is done to avoid conflict with a file of same name as the targets
# mentioned in this makefile.
#

.PHONY: help deps build $(TARGETS)
.DEFAULT_GOAL := build


.PHONY: license-check
license-check:
	@echo "Checking license header..."
	@licRes=$$(for file in $$(find . -type f -regex '.*\.sh\|.*\.go\|.*Docker.*\|.*\Makefile*' ! -path './vendor/*' ) ; do \
               awk 'NR<=5' $$file | grep -Eq "(Copyright|generated|GENERATED|License)" || echo $$file; \
       done); \
       if [ -n "$${licRes}" ]; then \
               echo "license header checking failed:"; echo "$${licRes}"; \
               exit 1; \
       fi
	@echo "Done checking license."

include Makefile.buildx.mk
