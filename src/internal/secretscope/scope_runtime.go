//go:build goexperiment.runtimesecret

package secretscope

import "runtime/secret"

// Do asks the experimental runtime to scrub transient machine state afterward.
func Do(fn func()) { secret.Do(fn) }
