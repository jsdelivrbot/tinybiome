#!/bin/bash
echo "building..."
cd $GIT_WORK_TREE
go build cmd/tinybiome.go -o tb
service nginx reload