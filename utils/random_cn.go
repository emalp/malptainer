package utils

import (
	"math/rand"
	"strings"
)

const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func GenerateRandomContainerName(length int) string {
	var sb strings.Builder
	sb.Grow(length)

	for i := 0; i < length; i++ {
		// rand.Intn returns a random number in the range [0, n)
		sb.WriteByte(charset[rand.Intn(len(charset))])
	}
	return "container-" + sb.String()
}
