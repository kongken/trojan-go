package fingerprint

import (
	"crypto/tls"
	"log/slog"

)

func ParseCipher(s []string) []uint16 {
	all := tls.CipherSuites()
	var result []uint16
	for _, p := range s {
		found := true
		for _, q := range all {
			if q.Name == p {
				result = append(result, q.ID)
				break
			}
			if !found {
				slog.Warn("invalid cipher suite", "suite", p)
			}
		}
	}
	return result
}
