package utils

import (
	"dainxor/atv/types"
	"fmt"
	"slices"
	"strings"

	"github.com/google/go-cmp/cmp"
)

/* Functional utilities for single values */

func Curry[T, R any](value T, fn func(T) R) func() R {
	return func() R {
		return fn(value)
	}
}
func Partial[T, U, R any](value T, fn func(T, U) R) func(arg U) R {
	return func(arg U) R {
		return fn(value, arg)
	}
}
func Extract(prefix string, text string, suffix string) string {
	size := len(text)

	if size < len(prefix)+len(suffix) {
		return ""
	}

	for i := range size {
		hasPrefix := prefix != "" && strings.HasPrefix(text[i:], prefix)
		if prefix == "" || hasPrefix {
			if hasPrefix {
				i += 1
			}

			for j := size; j >= 0; j-- {
				hasSuffix := suffix != "" && strings.HasSuffix(text[i:j], suffix)
				if suffix == "" || hasSuffix {
					if hasSuffix {
						j -= 1
					}

					return text[i:j]
				}
			}
		}
	}

	return ""
}

/* Functional utilities for slices */

// Removes elements that predicate returns false
func Filter[T any](slice []T, predicate func(T) bool) []T {
	result := make([]T, 0, len(slice))

	for _, value := range slice {
		if predicate(value) {
			result = append(result, value)
		}
	}
	return result
}
func Map[T, U any](slice []T, mapper func(T) U) []U {
	result := make([]U, 0, len(slice))

	for _, value := range slice {
		result = append(result, mapper(value))
	}
	return result
}

// MapE is a variant of Map that drops the elements for which the mapper returns an error.
// First option: Skip errors, keeps mapping even if any value returns an error. Returns the last error.
func MapE[T, U any](slice []T, mapper func(T) (U, error), options ...bool) ([]U, error) {
	result := make([]U, 0, len(slice))
	skipErrs := false

	if len(options) > 0 {
		skipErrs = options[0]
	}

	var lastError error
	lastError = nil
	for _, value := range slice {
		mappedValue, err := mapper(value)

		if err != nil {
			if skipErrs {
				lastError = err
				continue
			}

			return result, err
		} else {
			result = append(result, mappedValue)
		}
	}
	return result, lastError
}
func ForEach[T any](slice []T, action func(int, T)) {
	for i, value := range slice {
		action(i, value)
	}
}
func Reduce[T, U any](slice []T, reducer func(U, T) U, initial U) U {
	result := initial

	for _, value := range slice {
		result = reducer(result, value)
	}
	return result
}
func ReduceE[T, U any](slice []T, reducer func(U, T) (U, error), initial U) (U, error) {
	result := initial

	for _, value := range slice {
		if result, err := reducer(result, value); err != nil {
			return result, err
		}
	}
	return result, nil
}
func Any[T any](slice []T, predicate func(T) bool) bool {
	return slices.ContainsFunc(slice, predicate)
}
func All[T any](slice []T, predicate func(T) bool) bool {
	cmp := true
	for _, v := range slice {
		cmp = cmp && predicate(v)
	}

	return cmp
}
func Contains[S ~[]T, T any](slice S, value T) bool {
	return slices.ContainsFunc(slice, func(e T) bool {
		return cmp.Equal(e, value)
	})
}

/* Functional utilities for maps */

func DForEach[K comparable, V any](m map[K]V, action func(K, V)) {
	for k, v := range m {
		action(k, v)
	}
}
func DApply[K comparable, V any](m map[K]V, fn func(K, V) V) map[K]V {
	result := make(map[K]V, len(m))

	for k, v := range m {
		result[k] = fn(k, v)
	}
	return result
}
func DMap[K, NK comparable, V, NV any](m map[K]V, mapper func(K, V) (NK, NV)) map[NK]NV {
	result := make(map[NK]NV, len(m))

	for k, v := range m {
		newKey, newVal := mapper(k, v)
		result[newKey] = newVal
	}
	return result
}
func DMapE[K, NK comparable, V, NV any](m map[K]V, mapper func(K, V) (NK, NV, error), options ...bool) (map[NK]NV, error) {
	result := make(map[NK]NV, len(m))
	skipErrs := false

	if len(options) > 0 {
		skipErrs = options[0]
	}

	var lastError error
	lastError = nil
	for key, value := range m {
		mappedKey, mappedValue, err := mapper(key, value)

		if err != nil {
			if skipErrs {
				lastError = err
				continue
			}

			return result, err
		} else {
			result[mappedKey] = mappedValue
		}
	}
	return result, lastError
}
func DFlatten[K comparable, V, S any](m map[K]V, flattener func(K, V) S) []S {
	result := make([]S, 0, len(m))

	for k, v := range m {
		result = append(result, flattener(k, v))
	}

	return result
}
func DZip[K comparable, V1, V2 any](mainMap map[K]V1, zippedMap map[K]V2, defaultValue ...V2) map[K]types.Pair[V1, V2] {
	result := make(map[K]types.Pair[V1, V2], len(mainMap))

	for k, v1 := range mainMap {
		if v2, exists := zippedMap[k]; exists {
			result[k] = types.Pair[V1, V2]{First: v1, Second: v2}
		} else if len(defaultValue) > 0 {
			result[k] = types.Pair[V1, V2]{First: v1, Second: defaultValue[0]}
		} else {
			result[k] = types.Pair[V1, V2]{First: v1}
		}
	}
	return result
}
func DReduce[K comparable, V, R any](m map[K]V, reducer func(R, K, V) R, initial R) R {
	result := initial

	for k, v := range m {
		result = reducer(result, k, v)
	}
	return result
}

/* Functional utilities for strings */

func AsStrings(slice []any) []string {
	return Map(slice, func(e any) string { return fmt.Sprint(e) })
}
func TrimAll(slice []string, cutset string) []string {
	return Map(slice, func(e string) string {
		return strings.Trim(e, cutset)
	})
}
func Join(v []any, sep string) string {
	values := AsStrings(v)
	joinedArgs := strings.Join(values, sep)

	return strings.TrimSpace(joinedArgs)
}
func ToScreamingSnakeCase(input string) string {
	words := []rune{}
	for i, r := range input {
		if i > 0 && r == ' ' {
			words = append(words, '_')
		} else {
			words = append(words, r)
		}
	}
	return strings.TrimSpace(strings.ToUpper(string(words)))
}
