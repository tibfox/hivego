//go:build !debug

package hivego

// enableLogging is false when built without the "debug" build tag
// This disables logging for production builds
const enableLogging = false
