.PHONY: all build install test vet clean

all: build test

build:
	cd src && go build -o ../gx .

install:
	cd src && go install .

vet:
	cd src && go vet ./...

test:
	bash tests/run-all.sh

clean:
	rm -f gx
