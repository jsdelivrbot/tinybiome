#!/bin/bash
echo "fetching depedencies..."
go env
go get ./...
echo "building binary..."
go build -o tb cmd/tinybiome.go
echo "reloading nginx"
service nginx reload