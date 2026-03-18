BINARY := ems-bridge.exe

.PHONY: build build-utils run test clean

build:
	go build -o $(BINARY) .

build-utils:
	go build -o utils/encr.exe ./utils/
	go build -o http_client/http_client.exe ./http_client/

run:
	go run .

test:
	go test ./...

clean:
	rm -f $(BINARY) utils/encr.exe http_clien/http_client.exe
