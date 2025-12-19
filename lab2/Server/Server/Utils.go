package Server

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func formatSpeed(bps float64) string {
	const (
		KB = 1024.0
		MB = KB * 1024.0
		GB = MB * 1024.0
	)
	switch {
	case bps >= GB:
		return fmt.Sprintf("%.2f GB/s", bps/GB)
	case bps >= MB:
		return fmt.Sprintf("%.2f MB/s", bps/MB)
	case bps >= KB:
		return fmt.Sprintf("%.2f KB/s", bps/KB)
	default:
		return fmt.Sprintf("%.0f B/s", bps)
	}
}

func parseArgs() int {
	if len(os.Args) != 2 {
		fmt.Printf("usage: %s <port>\n", os.Args[0])
		os.Exit(1)
	}
	port, err := strconv.Atoi(os.Args[1])
	if err != nil || port <= 0 || port > 65535 {
		fmt.Println("invalid port")
		os.Exit(1)
	}
	return port
}

func createFile(fileInfo FileInfo) (string, *os.File, error) {
	safeFileName := fileInfo.Filename
	safeFileName = strings.ReplaceAll(safeFileName, "..", "_")
	safeFileName = strings.ReplaceAll(safeFileName, "/", "_")
	safeFileName = strings.ReplaceAll(safeFileName, "\\", "_")

	uploads := "uploads"
	err := os.MkdirAll(uploads, permissionBits)
	if err != nil {
		return "", nil, fmt.Errorf("Error create dir\n")
	}
	absUploads, err := filepath.Abs(uploads)
	if err != nil {
		return "", nil, fmt.Errorf("Error create dir\n")
	}
	pathFile := filepath.Join(absUploads, safeFileName)

	if _, err = os.Stat(pathFile); err == nil {
		ext := filepath.Ext(safeFileName)
		name := strings.TrimSuffix(safeFileName, ext)
		for i := 1; i < countSameFile; i++ {
			candidate := fmt.Sprintf("%s(%d)%s", name, i, ext)
			candPath := filepath.Join(absUploads, candidate)
			if _, err = os.Stat(candPath); os.IsNotExist(err) {
				pathFile = candPath
				break
			}
		}
	}

	file, err := os.Create(pathFile)
	if err != nil {
		return "", nil, fmt.Errorf("Error Cannot create file\n")
	}
	return pathFile, file, nil
}

func getAndParseHeader(reader *bufio.Reader) (FileInfo, error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		return FileInfo{}, fmt.Errorf("read error", err)
	}

	if line != "INFO\n" {
		return FileInfo{}, fmt.Errorf("Error Expected INFO\\n\n")
	}

	jsonLine, err := reader.ReadString('\n')
	if err != nil {
		return FileInfo{}, fmt.Errorf("Error Failed to read JSON\n")
	}
	jsonLine = strings.TrimRight(jsonLine, "\r\n")

	var fileInfo FileInfo
	err = json.Unmarshal([]byte(jsonLine), &fileInfo)
	if err != nil {
		return FileInfo{}, fmt.Errorf("Error Invalid JSON format\n")
	}

	if fileInfo.Filename == "" || fileInfo.Size <= 0 {
		return FileInfo{}, fmt.Errorf("Error Invalid file info\n")
	}
	return fileInfo, nil
}
