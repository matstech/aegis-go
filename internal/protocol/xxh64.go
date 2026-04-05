package protocol

import "github.com/shivakar/xxhash"

// XXH64Hex returns the lowercase hexadecimal XXH64 digest used by the Aegis server.
func XXH64Hex(payload []byte) string {
	hash := xxhash.NewXXHash64()
	_, _ = hash.Write(payload)
	return hash.String()
}
