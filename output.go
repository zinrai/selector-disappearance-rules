package main

import (
	"io"
	"os"
	"path/filepath"
)

const outputMode = 0o644

// Not written in place: a run that fails halfway, or a ruler that reads meanwhile, would find an
// empty or partial file, and a ruler takes an empty rule file as no rules rather than as an error.
func replaceFile(path string, write func(io.Writer) error) error {
	// Next to the target, not in the system's temporary directory: a rename is atomic only
	// within one filesystem.
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())

	if err := fill(tmp, write); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func fill(tmp *os.File, write func(io.Writer) error) error {
	if err := write(tmp); err != nil {
		return err
	}
	// Not left at the mode CreateTemp gives: that one is readable by the owner alone, and the
	// ruler reading the file may run as someone else.
	if err := tmp.Chmod(outputMode); err != nil {
		return err
	}
	return tmp.Sync()
}
