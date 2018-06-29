#!/bin/bash
echo "fetching depedencies in..."
pwd

go env
go get ./...
echo "building binary..."
go build -o /root/tb cmd/tinybiome.go
echo "adding service"
cp cmd/tbserver.service /etc/systemd/system/
echo "restarting tbserver service"
systemctl enable tbserver.service
systemctl restart tbserver.service
echo "copying nginx"
cp cmd/nginx.conf /etc/nginx/sites-enabled/tinybio.me.conf
echo "reloading nginx"
service nginx reload