package config

import (
	"os"
	"path/filepath"
)

type ConfigFS interface {
	fileExists(path string) (bool, error)
	makePath(path string) error
}

type configFS struct{}

var _ ConfigFS = &configFS{}

func newConfigFS() *configFS {
	return &configFS{}
}

func (fs *configFS) fileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (fs *configFS) makePath(path string) error {
	dir := filepath.Dir(path)

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err = os.MkdirAll(dir, os.ModePerm)
		if err != nil {
			return err
		}
	}

	return nil
}
