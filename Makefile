.PHONY: check check-internal

check:
	@log="$$(mktemp)"; \
	trap 'rm -f "$$log"' EXIT; \
	if $(MAKE) --no-print-directory check-internal >"$$log" 2>&1; then \
		printf 'true\n'; \
	else \
		printf 'false\n'; \
		tail -n 80 "$$log"; \
		exit 1; \
	fi

check-internal:
	@python3 scripts/validate_agents_links.py .
	@python3 scripts/validate_agents_landmarks.py .
	@nixfmt --check flake.nix
	@nix flake check
	@fzz check
	@go build ./...
	@golangci-lint run
	@go test ./...
