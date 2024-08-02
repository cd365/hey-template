all: fmt linux

program=hey-template

fmt:
	@./script/fmt.bash

linux:
	@./script/linux.bash

upx:
	@upx -9 ${program}

clean:
	@rm -f ${program}
