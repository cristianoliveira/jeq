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
	@nixfmt --check flake.nix
	@nix flake check
	@fzz check
