TARGETS := linux/amd64 linux/arm64 linux/arm windows/amd64 windows/arm64 darwin/amd64 darwin/arm64

.PHONY: test integration release

test:
	go vet ./...
	go test -race ./...

integration:
	docker compose -f test/integration/docker-compose.yml up --build --abort-on-container-exit --exit-code-from tester; rc=$$?; \
	docker compose -f test/integration/docker-compose.yml down -v; exit $$rc

release:
	rm -rf dist && mkdir -p dist
	for t in $(TARGETS); do \
	  os=$${t%/*}; arch=$${t#*/}; ext=; [ $$os = windows ] && ext=.exe; \
	  CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -trimpath -ldflags "-s -w" \
	    -o dist/devupdater-$$os-$$arch$$ext ./cmd/devupdater || exit 1; \
	done
