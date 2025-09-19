package e2e

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"strconv"
	"strings"
	"time"
)

func totpCode(base32Secret string, t time.Time) string {
	secret, _ := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(base32Secret))
	counter := uint64(t.Unix() / 30)
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], counter)

	mac := hmac.New(sha1.New, secret)
	mac.Write(buf[:])
	sum := mac.Sum(nil)

	offset := sum[len(sum)-1] & 0x0F
	bin := (uint32(sum[offset])&0x7F)<<24 |
		(uint32(sum[offset+1])&0xFF)<<16 |
		(uint32(sum[offset+2])&0xFF)<<8 |
		(uint32(sum[offset+3]) & 0xFF)
	code := int(bin % 1000000)
	return format6(code)
}

func format6(n int) string {
	s := "000000" + strconv.Itoa(n)
	return s[len(s)-6:]
}
