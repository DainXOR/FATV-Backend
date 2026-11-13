//go:build !goexperiment.runtimesecret

package secretscope

// Do executes fn normally when Go's experimental runtime secret support is off.
func Do(fn func()) { fn() }
