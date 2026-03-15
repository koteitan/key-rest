module github.com/koteitan/key-rest

go 1.24.0

require (
	github.com/koteitan/key-rest/go v0.0.0
	golang.org/x/crypto v0.48.0
	golang.org/x/term v0.40.0
)

require (
	github.com/andybalholm/brotli v1.2.0 // indirect
	golang.org/x/sys v0.41.0 // indirect
)

replace github.com/koteitan/key-rest/go => ./clients/go
