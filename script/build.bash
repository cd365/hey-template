#!/bin/bash

program=$1
buildAt=$(date +'%Y%m%d%H%M%S')
commitId=$(git log --pretty=oneline -n 1 | awk '{print $1}')

# 构建动态命令
build="GOOS=linux GOARCH=amd64 CGO_ENABLED=0 CC=musl-gcc go build -ldflags '-s -w -extldflags \"-static\" -X root/values.BuildAt=${buildAt} -X root/values.CommitId=${commitId}' -o ${program}"

go mod tidy
rm -f ${program}

# 执行动态命令
eval $build
