.MAIN: build

build:
	go build -o hosts

build-strip:
	go build -ldflags "-s -w" -o hostsqa

clean:
	rm 2>/dev/null hostsqa hosts | true
