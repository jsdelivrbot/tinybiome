#!/bin/bash
echo "fetching depedencies in..."
pwd

go env
go get ./...
echo "building binary..."
go build -o /root/tb cmd/tinybiome.go
cp ui /root/ui
echo "adding service"
cp cmd/tbserver.service /etc/systemd/system/
echo "restarting tbserver service"
systemctl enable tbserver.service
systemctl restart tbserver.service
echo "copying server conf"
cp cmd/http.conf /etc/nginx/sites-enabled/tbbackend
echo "reloading nginx"
service nginx reload