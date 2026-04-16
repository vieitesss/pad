package appfs

import "os"

func ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func WriteFile(path string, data []byte, perm os.FileMode) error {
	if err := os.MkdirAll(getDir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, perm)
}

func getDir(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			return path[:i]
		}
	}
	return "."
}
