program=$(shell /bin/bash ./script/program.bash)

all: fmt linux

fmt:
	./script/fmt.bash

linux:
	./script/linux.bash ${program}

upx:
	upx -9 ${program}

clean:
	rm -f ${program}
