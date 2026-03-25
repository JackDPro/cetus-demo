// Usage: go run storage/optimus_gen.go [prime]
//
// Generate OPTIMUS_PRIME, OPTIMUS_INVERSE, OPTIMUS_RANDOM for .env
// Prime numbers can be found at: http://primes.utm.edu/lists/small/millions/
//
// Example:
//   go run storage/optimus_gen.go 104393867

package main

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
	"strconv"
)

const maxInt = uint64(1<<31 - 1) // 2,147,483,647

func modInverse(prime uint64) uint64 {
	p := big.NewInt(int64(prime))
	max := big.NewInt(int64(maxInt + 1))
	var i big.Int
	return i.ModInverse(p, max).Uint64()
}

func randN(n int64) uint64 {
	upper := big.NewInt(n)
	r, _ := rand.Int(rand.Reader, upper)
	return r.Uint64() + 1
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: go run storage/optimus_gen.go <prime>")
		fmt.Fprintln(os.Stderr, "Prime numbers: http://primes.utm.edu/lists/small/millions/")
		os.Exit(1)
	}

	prime, err := strconv.ParseUint(os.Args[1], 10, 64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid prime: %s\n", os.Args[1])
		os.Exit(1)
	}

	inverse := modInverse(prime)
	random := randN(int64(maxInt - 1))

	fmt.Printf("OPTIMUS_PRIME=%d\n", prime)
	fmt.Printf("OPTIMUS_INVERSE=%d\n", inverse)
	fmt.Printf("OPTIMUS_RANDOM=%d\n", random)
}