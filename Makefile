program=$(shell /bin/bash ./script/program.bash)

all: wire fmt build

wire:
	@cd initial;wire;cd -

fmt:
	@./script/fmt.bash

build:
	@./script/build.bash ${program}

upx:
	@upx -9 ${program}

clean:
	rm -f ${program}
