all: fmt linux

program=hey-template

fmt:
	@./bash/fmt.bash

linux:
	@./bash/linux.bash

upx:
	@upx -9 ${program}

clean:
	@rm -f ${program}
