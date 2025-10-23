package Server

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type FileInfo struct {
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
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

func Server() {
	port := parseArgs()

	addr := ":" + strconv.Itoa(port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Println("Error listening:", err.Error())
		os.Exit(1)
	}
	defer listener.Close()
	fmt.Println("Server is listening on", port)

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Error accepting:", err.Error())
			os.Exit(1)
		}
		fmt.Println("Connected with", conn.RemoteAddr().String())
		go handleConn(conn)
	}
}

func handleConn(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)

	line, err := reader.ReadString('\n')
	if err != nil {
		fmt.Println("read error:", err)
		return
	}

	if line != "INFO\n" {
		conn.Write([]byte("Error Expected INFO\\n\n"))
		return
	}

	jsonLine, err := reader.ReadString('\n')
	if err != nil {
		conn.Write([]byte("Error Failed to read JSON\n"))
		return
	}
	jsonLine = strings.TrimRight(jsonLine, "\r\n")

	var fileInfo FileInfo
	err = json.Unmarshal([]byte(jsonLine), &fileInfo)
	if err != nil {
		conn.Write([]byte("Error Invalid JSON format\n"))
		return
	}

	if fileInfo.Filename == "" || fileInfo.Size <= 0 {
		conn.Write([]byte("Error Invalid file info\n"))
		return
	}

	safeFileName := fileInfo.Filename
	safeFileName = strings.ReplaceAll(safeFileName, "..", "_")
	safeFileName = strings.ReplaceAll(safeFileName, "/", "_")
	safeFileName = strings.ReplaceAll(safeFileName, "\\", "_")

	uploads := "uploads"
	err = os.MkdirAll(uploads, 0755)
	if err != nil {
		conn.Write([]byte("Error create dir\n"))
		return
	}
	absUploads, err := filepath.Abs(uploads)
	if err != nil {
		conn.Write([]byte("Error create dir\n"))
		return
	}
	pathFile := filepath.Join(absUploads, safeFileName)

	if _, err = os.Stat(pathFile); err == nil {
		ext := filepath.Ext(safeFileName)
		name := strings.TrimSuffix(safeFileName, ext)
		for i := 1; i < 100; i++ {
			candidate := fmt.Sprintf("%s(%d)%s", name, i, ext)
			candPath := filepath.Join(absUploads, candidate)
			if _, err = os.Stat(candPath); os.IsNotExist(err) {
				pathFile = candPath
				break
			}
		}
	}

	//_, err = conn.Write([]byte("OK\n"))
	//if err != nil {
	//	fmt.Println("failed to write OK:", err)
	//	return
	//	}

	file, err := os.Create(pathFile)
	if err != nil {
		conn.Write([]byte("ERROR Cannot create file\n"))
		return
	}
	defer file.Close()

	receivedBytes, err := io.CopyN(file, reader, fileInfo.Size)
	if err != nil {
		fmt.Printf("Error receiving file data: %v\n", err)
		conn.Write([]byte("Error Failed to receive file data\n"))

		file.Close()
		os.Remove(pathFile)
		return
	}

	if receivedBytes == fileInfo.Size {
		conn.Write([]byte("STATUS OK\n"))
		fmt.Printf("File %s received successfully (%d bytes)\n", fileInfo.Filename, receivedBytes)
	} else {
		conn.Write([]byte("STATUS FAIL\n"))
		fmt.Printf("File size mismatch: expected %d, received %d\n", fileInfo.Size, receivedBytes)

		file.Close()
		os.Remove(pathFile)
	}
}
