.PHONY: all test

all: test stmps CHANGELOG.md

VERSION != git describe --tags HEAD

stmps:
	go build -ldflags="-X main.Version=$(VERSION)" -o stmps .

CHANGELOG.md:
	git cliff -o CHANGELOG.md

test:
	go test ./...
	markdownlint README.md
	golangci-lint run
