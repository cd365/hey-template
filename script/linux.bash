#!/bin/bash

program=hey-template
buildTime=$(date +'%Y%m%d%H%M%S')
commitHash=$(git log --pretty=oneline -n 1 | awk '{print $1}')

# 构建动态命令
build="GOOS=linux GOARCH=amd64 CGO_ENABLED=0 CC=musl-gcc go build -ldflags '-s -w -extldflags \"-static\" -X main.BuildTime=${buildTime} -X main.CommitHash=${commitHash}' -o ${program}"

go mod tidy
rm -f ${program}
# 执行动态命令
eval $build
