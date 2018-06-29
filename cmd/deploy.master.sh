#!/bin/bash
echo "fetching depedencies in..."
pwd

go env
go get ./...
echo "building binary..."
go build -o tb cmd/tinybiome.go
echo "reloading nginx"
service nginx reload