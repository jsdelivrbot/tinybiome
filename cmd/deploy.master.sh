#!/bin/bash
cd $GIT_WORK_TREE
pwd
echo "fetching depedencies..."
go get ./...
echo "building binary..."
go build -o tb cmd/tinybiome.go
echo "reloading nginx"
service nginx reload