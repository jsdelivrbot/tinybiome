#!/bin/bash
echo "fetching depedencies in..."
pwd

go env
go get ./...
echo "building binary..."
go build -o /root/tb cmd/tinybiome.go
echo "adding service"
cp cmd/tbserver /etc/init.d/tbserver
echo "restarting tbserver service"
service tbserver stop
service tbserver start
echo "reloading nginx"
service nginx reload