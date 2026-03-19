-include config.mk

ifeq ($(OS),Windows_NT)
    EXT := .exe
else
    EXT :=
endif

BINARY := ems-bridge$(EXT)

BUILD_TAGS :=
ifeq ($(TIBCO_EMS),1)
    BUILD_TAGS := -tags tibco
endif

.PHONY: build build-utils run test clean

build:
	go build $(BUILD_TAGS) -o $(BINARY) .

build-utils:
	go build -o utils/encr$(EXT) ./utils/
	go build -o http_client/http_client$(EXT) ./http_client/

run:
	go run $(BUILD_TAGS) .

test:
	go test $(BUILD_TAGS) ./...

clean:
	rm -f $(BINARY) utils/encr$(EXT) http_client/http_client$(EXT)
