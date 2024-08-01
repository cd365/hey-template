#!/bin/bash
# format source code of go
for tmp in $(find . -name "*.go");do go fmt "${tmp}";done