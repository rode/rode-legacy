package test

import (
	"crypto/sha256"
	"encoding/hex"
	krand "k8s.io/apimachinery/pkg/util/rand"
	"math/rand"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
	rand.Seed(time.Now().UnixNano())
}

func RandomStringSlice() []string {
	var result []string

	for i := 0; i < krand.Intn(5)+1; i++ {
		result = append(result, krand.String(10))
	}

	return result
}

func RandomStringSliceSubset(s []string) []string {
	var result []string

	for _, v := range s {
		if RandomBool() {
			result = append(result, v)
		}
	}

	return result
}

func RandomBool() bool {
	return rand.Float32() < 0.5
}

func RandomAWSAccountNumber() string {
	const chars = "1234567890"

	ar := make([]byte, 12)
	for i := range ar {
		ar[i] = chars[rand.Intn(len(chars))]
	}

	return string(ar)
}

func CreateTestSha256(args ...string) string {
	hash := sha256.New()

	for _, s := range args {
		hash.Write([]byte(s))
	}

	return hex.EncodeToString(hash.Sum(nil))
}
