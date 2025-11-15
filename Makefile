.PHONY: format
format:
	grep -L -R "^// Code generated .* DO NOT EDIT\.$$" --exclude-dir=.git --include="*.go" . | xargs go tool gofumpt -w

.PHONY: lint
lint:
	go tool honnef.co/go/tools/cmd/staticcheck ./...
