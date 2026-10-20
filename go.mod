module latere.ai/x/agon

go 1.27.0

toolchain go1.27.2

require (
	latere.ai/x/pkg v0.90.2
	latere.ai/x/sdk-go v0.0.0-00010101000000-000000000000
)

require (
	github.com/goccy/go-yaml v1.19.2 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	golang.org/x/mod v0.41.0 // indirect
	golang.org/x/sync v0.23.0 // indirect
	golang.org/x/tools v0.51.0 // indirect
	latere.ai/x/ci-gate v0.54.0 // indirect
)

tool latere.ai/x/ci-gate/cmd/lateregate

// The SDK's llm package is unreleased; this points at its working tree until
// it is tagged.
replace latere.ai/x/sdk-go => ../sdk-llm
