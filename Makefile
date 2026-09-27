# Copyright 2020 Changkun Ou. All rights reserved.

# changkun.de/research is a plain folder a static file server serves.
# `make` renders index.html and the talk links into this working tree;
# `make deploy` copies the site into the folder the server serves.

NAME=research
BUILD_TIME = $(shell date '+%Y-%m-%d')
GIT_COMMIT=$(shell git rev-parse --short HEAD)
BUILD_FLAGS = -ldflags "-X main.BuildTime=$(BUILD_TIME) -X main.BuildHash=$(GIT_COMMIT)"
WWW ?= /www/changkun.de/research

all:
	go build $(BUILD_FLAGS)
	./$(NAME)
test:
	go test ./...
deploy: test all
	mkdir -p $(WWW)
	rsync -a --delete --exclude /assets/index.html index.html assets papers talks teach theses $(WWW)/
clean:
	rm -f $(NAME) index.html
	find talks -maxdepth 1 -type l -delete
