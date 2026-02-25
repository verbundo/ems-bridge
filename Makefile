BINARY := ems-bridge

.PHONY: build build-utils run test clean

build:
	go build -o $(BINARY) .

build-utils:
	go build -o utils/encr ./utils/

run:
	go run .

test:
	go test ./...

clean:
	rm -f $(BINARY) utils/encr
