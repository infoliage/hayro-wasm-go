.PHONY: lint test update-wasm

lint:
	golangci-lint run

# Rebuilds hayro-wasm-bridge from the sibling checkout at ../hayro-wasm-bridge
# and copies the resulting .wasm in, for go:embed to pick up (see hayro.go).
# Run this after pulling in changes to hayro-wasm-bridge.
update-wasm:
	cd ../hayro-wasm-bridge && cargo build --target wasm32-unknown-unknown --release
	cp ../hayro-wasm-bridge/target/wasm32-unknown-unknown/release/hayro_wasm_bridge.wasm wasm/hayro_wasm_bridge.wasm

test:
	go test ./...

