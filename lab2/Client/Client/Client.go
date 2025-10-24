package Client

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
)

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

	file, err := os.Open(filePath)
	if err != nil {
		fmt.Println("Error opening file:", err.Error())
		return
	}
	defer file.Close()

	buff := make([]byte, buffSize)
	var currWritten int64 = 0

	for currWritten < fileSize {
		remains := fileSize - currWritten
		nToRead := buffSize
		if int64(nToRead) > remains {
			nToRead = int(remains)
		}

		n, readErr := file.Read(buff[:nToRead])
		if n > 0 {
			written := 0
			for written < n {
				wn, writeErr := conn.Write(buff[written:n])
				if writeErr != nil {
					fmt.Printf("Error sending file: %v (written %d bytes)\n", writeErr, currWritten+int64(written))
					return
				}
				written += wn
			}
			currWritten += int64(n)
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			fmt.Printf("Error reading file: %v\n", readErr)
			return
		}
	}

	tcpConn, ok := conn.(*net.TCPConn)
	if ok {
		err := tcpConn.CloseWrite()
		if err != nil {
			fmt.Println("Error close sending:", err.Error())
			return
		}
	}

	reader := bufio.NewReader(conn)
	status, err := reader.ReadString('\n')
	if err != nil {
		fmt.Println("Error reading status:", err.Error())
		return
	}

	fmt.Printf("Server response: %s", status)
}
