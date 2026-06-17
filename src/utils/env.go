package utils

import (
	"fmt"
	"os"
	"strconv"
)

func GetEnvInt(key string, fallback int) int {
	v, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return i
}

func GetEnvFloat(key string, fallback float64) float64 {
	v, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return fallback
	}
	return f
}

func GetEnvBool(key string, fallback bool) bool {
	v, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}

func GetEnvString(key string, fallback string) string {
	v, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}

	return v
}

// -------------------------------------------------

func GetEnvIntOrErr(key string) (int, error) {
	v, ok := os.LookupEnv(key)
	if !ok {
		return 0, fmt.Errorf("The env key %s does not exists", key)
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("Error parsing to int the env value: %s", v)
	}

	return i, nil
}

func GetEnvFloatOrErr(key string) (float64, error) {
	v, ok := os.LookupEnv(key)
	if !ok {
		return 0, fmt.Errorf("The env key %s does not exists", key)
	}
	i, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0, fmt.Errorf("Error parsing to float the env value: %s", v)
	}

	return i, nil
}

func GetEnvBoolOrErr(key string) (bool, error) {
	v, ok := os.LookupEnv(key)
	if !ok {
		return false, fmt.Errorf("The env key %s does not exists", key)
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("Error parsing to float the env value: %s", v)
	}

	return b, nil
}

func GetEnvStringOrErr(key string) (string, error) {
	v, ok := os.LookupEnv(key)
	if !ok {
		return "", fmt.Errorf("The env key %s does not exists", key)
	}

	return v, nil
}
