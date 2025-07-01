.MAIN: build

build:
	go build -o hostsc

build-strip:
	go build -ldflags "-s -w" -o hostsc

clean:
	rm 2>/dev/null hostsc hosts | true
