package Client

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"unicode/utf8"
)

type FileInfo struct {
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
}

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
	const maxSize int64 = 1 << 40
	if info.Size() > maxSize {
		return "", 0, fmt.Errorf("file size exceeds 1 TB: %d", info.Size())
	}

	base := filepath.Base(filePath)
	if !utf8.ValidString(base) {
		return "", 0, fmt.Errorf("filename is not valid UTF-8: %q", base)
	}
	if len([]byte(base)) > 4096 {
		return "", 0, fmt.Errorf("filename encoded in UTF-8 exceeds 4096 bytes")
	}

	return base, info.Size(), nil
}

func parseArgs() (string, string, int, error) {
	if len(os.Args) != 4 {
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

func Client() {
	filePath, ip, port, err := parseArgs()
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}

	filename, fileSize, err := validateFilePath(filePath)
	if err != nil {
		fmt.Println("File validation error:", err)
		os.Exit(1)
	}

	addr := net.JoinHostPort(ip, strconv.Itoa(port))
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		fmt.Println("Error connecting:", err)
		os.Exit(1)
	}
	defer conn.Close()

	fileInfo := FileInfo{Filename: filename, Size: fileSize}
	jsonData, err := json.Marshal(fileInfo)
	if err != nil {
		fmt.Println("Error creating JSON:", err)
		return
	}

	infoMessage := "INFO\n" + string(jsonData) + "\n"
	_, err = conn.Write([]byte(infoMessage))
	if err != nil {
		fmt.Println("Error sending file info:", err)
		return
	}

	reader := bufio.NewReader(conn)
	//response, err := reader.ReadString('\n')
	//if err != nil {
	//	fmt.Println("Error reading server response:", err)
	//	return
	//}
	//
	//if response != "OK\n" {
	//	fmt.Printf("Server rejected request: %s", response)
	//	return
	//}

	file, err := os.Open(filePath)
	if err != nil {
		fmt.Println("Error opening file:", err.Error())
		return
	}
	defer file.Close()

	written, err := io.CopyN(conn, file, fileSize)
	if err != nil {
		fmt.Printf("Error sending file data: %v (written %d bytes)\n", err, written)
		return
	}

	tcpConn, ok := conn.(*net.TCPConn)
	if ok {
		err := tcpConn.CloseWrite()
		if err != nil {
			fmt.Println("Error close sending:", err.Error())
			return
		}
	}

	status, err := reader.ReadString('\n')
	if err != nil {
		fmt.Println("Error reading status:", err.Error())
		return
	}

	fmt.Printf("Server response: %s", status)
}
