TARGETS := linux/amd64 linux/arm64 linux/arm windows/amd64 windows/arm64 darwin/amd64 darwin/arm64

.PHONY: test integration release ui ui-build e2e e2e-check

test:
	go vet ./...
	go test -race ./...

integration:
	docker compose -f test/integration/docker-compose.yml up --build --abort-on-container-exit --exit-code-from tester; rc=$$?; \
	docker compose -f test/integration/docker-compose.yml down -v; exit $$rc

ui:
	cd ui && npm ci && npm run build

# Install UI dependencies only when missing, then build the embedded bundle.
ui-build:
	cd ui && ([ -d node_modules ] || npm ci) && npm run build

# Prerequisite: Chromium for Playwright. Install it once with
# `cd ui && npx playwright install chromium`; make never downloads it.
e2e-check:
	@cd ui && [ -d node_modules/@playwright/test ] || { echo "e2e: run 'npm ci' in ui/ first"; exit 1; }
	@cd ui && npx playwright install --dry-run chromium | sed -n 's/^ *Install location: *//p' | while read -r d; do \
	  [ -d "$$d" ] || { echo "e2e: Playwright chromium is missing. Run 'npx playwright install chromium' from ui/."; exit 1; }; \
	done

e2e: e2e-check ui-build
	cd ui && npx playwright test

release: ui
	rm -rf dist && mkdir -p dist
	for t in $(TARGETS); do \
	  os=$${t%/*}; arch=$${t#*/}; ext=; [ $$os = windows ] && ext=.exe; \
	  CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -trimpath -ldflags "-s -w" \
	    -o dist/devupdater-$$os-$$arch$$ext ./cmd/devupdater || exit 1; \
	done
