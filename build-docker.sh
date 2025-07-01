#!/bin/bash

docker run --rm -v "${PWD}:/app" --workdir='/app' golang:1.20.2-buster go build -buildvcs=false -o hostsc-docker
