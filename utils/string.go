package utils

import (
	"strings"
	"unsafe"
)

// Pascal 帕斯卡命名
func Pascal(str string) string {
	if str == "" {
		return ""
	}
	length := len(str)
	tmp := make([]byte, 0, length)
	next2upper := true
	for i := 0; i < length; i++ {
		if str[i] == '_' {
			next2upper = true
			continue
		}
		if next2upper && str[i] >= 'a' && str[i] <= 'z' {
			tmp = append(tmp, str[i]-32)
		} else {
			tmp = append(tmp, str[i])
		}
		next2upper = false
	}
	return string(tmp[:])
}

// PascalFirstLower 首字母小写的帕斯卡命名
func PascalFirstLower(str string) string {
	if str == "" {
		return ""
	}
	str = Pascal(str)
	return strings.ToLower(str[0:1]) + str[1:]
}

// Underline 下划线命名
func Underline(str string) string {
	if str == "" {
		return ""
	}
	length := len(str)
	tmp := make([]byte, 0, length)
	for i := 0; i < length; i++ {
		if str[i] >= 'A' && str[i] <= 'Z' {
			if i > 0 {
				tmp = append(tmp, '_')
			}
			tmp = append(tmp, str[i]+32)
		} else {
			tmp = append(tmp, str[i])
		}
	}
	return *(*string)(unsafe.Pointer(&tmp))
}

func Upper(str string) string {
	return strings.ToUpper(str)
}

func Lower(str string) string {
	return strings.ToLower(str)
}
