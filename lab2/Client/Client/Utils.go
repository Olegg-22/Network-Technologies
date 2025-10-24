package Client

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"unicode/utf8"
)

func validateFilePath(filePath string) (baseName string, size int64, err error) {
	info, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", 0, fmt.Errorf("file does not exist: %s", filePath)
		}
		return "", 0, fmt.Errorf("file access error: %v", err)
	}
	if !info.Mode().IsRegular() {
		return "", 0, fmt.Errorf("path is not a regular file: %s", filePath)
	}
	if info.Size() > maxSize {
		return "", 0, fmt.Errorf("file size exceeds 1 TB: %d", info.Size())
	}

	base := filepath.Base(filePath)
	if !utf8.ValidString(base) {
		return "", 0, fmt.Errorf("filename is not valid UTF-8: %q", base)
	}
	if len([]byte(base)) > maxLenFileName {
		return "", 0, fmt.Errorf("filename encoded in UTF-8 exceeds 4096 bytes")
	}

	return base, info.Size(), nil
}

func parseArgs() (string, string, int, error) {
	if len(os.Args) != countArgument {
		return "", "", 0, fmt.Errorf("usage: %s <file_path> <server_ip> <server_port>", os.Args[0])
	}
	filePath := os.Args[1]
	ip := os.Args[2]
	port, err := strconv.Atoi(os.Args[3])
	if err != nil {
		return "", "", 0, fmt.Errorf("invalid port: %v", err)
	}
	return filePath, ip, port, nil
}
