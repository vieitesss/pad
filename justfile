set shell := ["bash", "-euo", "pipefail", "-c"]

_default:
	just --list

build:
	go build ./...

test:
	go test ./...

release-tag version:
	version="{{version}}"; \
	if [[ ! "$version" =~ ^v?[0-9]+\.[0-9]+\.[0-9]+$ ]]; then \
		echo "version must look like 0.1.0 or v0.1.0" >&2; \
		exit 1; \
	fi; \
	version="${version#v}"; \
	git tag "v${version}"; \
	git push origin "v${version}"
