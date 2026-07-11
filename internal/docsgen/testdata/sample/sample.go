// Package sample is a tiny fixture package used by docsgen's tests.
// It exists only to give the generator a package doc comment and one
// exported identifier to render.
package sample

// Greet returns a fixed greeting. It exists only to give sample an
// exported identifier.
func Greet() string {
	return "hello"
}
