.PHONY: build test-smoke test-ha lint

build:
	go build ./...

test-smoke:
	DISABLE_DOTENV=1 go test ./test/unit/... -count=1
	DISABLE_DOTENV=1 go test ./test/e2e -run '00_|01_|02_|04_|08_' -count=1 -timeout=6m

test-ha:
	E2E_SKIP_GLOBAL_SERVER=1 DISABLE_DOTENV=1 go test ./test/e2e -run 'Test_40_|Test_41_|Test_42_' -count=1 -timeout=8m

lint:
	golangci-lint run || true
