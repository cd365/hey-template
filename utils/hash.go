package utils

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

func Sha256(str string) string {
	bts := []byte(str)
	hash := sha256.Sum256(bts)
	return hex.EncodeToString(hash[:])
}

func Md5(str string) string {
	has := md5.Sum([]byte(str))
	return fmt.Sprintf("%x", has)
}
