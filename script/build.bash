#!/bin/bash

program=$1
buildAt=$(date +'%Y%m%d%H%M%S')
commitId=$(git log --pretty=oneline -n 1 | awk '{print $1}')

# musl-gcc => www.musl-libc.org

# 构建动态命令
build="GOOS=linux GOARCH=amd64 CGO_ENABLED=0 CC=musl-gcc go build -ldflags '-s -w -extldflags \"-static\" -X github.com/cd365/hey-template/values.BuildAt=${buildAt} -X github.com/cd365/hey-template/values.CommitId=${commitId}' -o ${program}"

go mod tidy
rm -f ${program}

# 执行动态命令
eval $build
