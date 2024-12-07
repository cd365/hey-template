package utils

import (
	"os"
	"path/filepath"
)

// RemoveCreateFile 如果文件存在, 先删除再创建
func RemoveCreateFile(filename string) (*os.File, error) {
	dir := filepath.Dir(filename)
	if _, err := os.Stat(dir); err != nil {
		if err = os.MkdirAll(dir, 0755); err != nil {
			return nil, err
		}
	}
	if _, err := os.Stat(filename); err == nil {
		if err = os.Remove(filename); err != nil {
			return nil, err
		}
	}
	return os.Create(filename)
}
