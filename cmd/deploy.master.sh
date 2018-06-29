#!/bin/bash
echo "building..."
cd $GIT_WORK_TREE
pwd
go build -o tb cmd/tinybiome.go
service nginx reload