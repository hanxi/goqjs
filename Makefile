BINARY = goqjs
GOFLAGS =

# Platform-specific linker flags to increase C thread stack size.
# Obfuscated JavaScript can cause deep C recursion during parsing,
# exceeding the default 8MB CGO thread stack.
UNAME_S := $(shell uname -s)
ifeq ($(UNAME_S),Darwin)
    # macOS: set main thread stack to 32MB
    EXTLDFLAGS = -Wl,-stack_size,0x2000000
else ifeq ($(UNAME_S),Linux)
    # Linux: set default thread stack to 32MB
    EXTLDFLAGS = -Wl,-z,stacksize=33554432
endif

LDFLAGS = -ldflags "-extldflags '$(EXTLDFLAGS)'"

.PHONY: build test clean

build:
	go build $(LDFLAGS) -o $(BINARY) ./cmd/goqjs/

test:
	go test ./qjs/ -v -count=1

clean:
	rm -f $(BINARY)
