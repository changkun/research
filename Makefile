# Copyright 2020 Changkun Ou. All rights reserved.

NAME=research
VERSION = $(shell git describe --always --tags)
BUILD_TIME = $(shell date '+%Y-%m-%d')
GIT_COMMIT=$(shell git rev-parse --short HEAD)
BUILD_FLAGS = -ldflags "-X main.BuildTime=$(BUILD_TIME) -X main.BuildHash=$(GIT_COMMIT)"

all:
	go build $(BUILD_FLAGS)
build:
	CGO_ENABLED=0 GOOS=linux go build $(BUILD_FLAGS)
	docker build -t $(NAME):$(VERSION) -t $(NAME):latest .
up:
	docker-compose up -d
down:
	docker-compose down
clean: down
	rm -rf $(NAME)
	docker rmi -f $(shell docker images -f "dangling=true" -q) 2> /dev/null; true
	docker rmi -f $(NAME):latest $(NAME):$(VERSION) 2> /dev/null; true
