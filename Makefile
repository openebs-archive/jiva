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


.dapper:
	@echo Downloading dapper
	@curl -sL https://releases.rancher.com/dapper/latest/dapper-`uname -s`-`uname -m` > .dapper.tmp
	@@chmod +x .dapper.tmp
	@./.dapper.tmp -v
	@mv .dapper.tmp .dapper

$(TARGETS): .dapper
	./.dapper $@

trash: .dapper
	./.dapper -m bind trash

trash-keep: .dapper
	./.dapper -m bind trash -k


deps: _build_check_go _build_check_docker trash
	@echo ""
	@echo "INFO:\tverifying dependencies for jiva ..."

_install_trash:
	go get -u github.com/rancher/trash

_run_ci:
	@echo ""
	@echo "INFO:\t..... run ci over jiva image"
	@echo ""
	sudo ./ci/start_init_test.sh

_push_image:
	cd $(GOPATH)/src/github.com/openebs/jiva && ./scripts/push

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

build: deps _install_trash ci _run_ci _push_image

#
# This is done to avoid conflict with a file of same name as the targets
# mentioned in this makefile.
#

.PHONY: help deps build $(TARGETS)
.DEFAULT_GOAL := build
