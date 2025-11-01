package Server

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"sync/atomic"
	"time"
)

func handleConn(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)

	fileInfo, err := getAndParseHeader(reader)
	if err != nil {
		_, err = conn.Write([]byte(err.Error()))
		if err != nil {
			fmt.Println("Error writing to connection:", err.Error())
		}
		return
	}

	pathFile, file, err := createFile(fileInfo)
	if err != nil {
		_, err = conn.Write([]byte(err.Error()))
		if err != nil {
			fmt.Println("Error writing to connection:", err.Error())
		}
		return
	}
	defer file.Close()

	var totalBytes int64 = 0
	var bytesSinceLast int64 = 0
	startTime := time.Now()
	lastPrint := startTime

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	done := make(chan struct{})

	go func(remoteAddr string, fileInfo FileInfo) {
		for {
			select {
			case tickTime := <-ticker.C:
				b := atomic.SwapInt64(&bytesSinceLast, 0)
				inst := float64(b) / tickTime.Sub(lastPrint).Seconds()
				avg := float64(atomic.LoadInt64(&totalBytes)) / time.Since(startTime).Seconds()
				loading := float64(atomic.LoadInt64(&totalBytes)) / float64(fileInfo.Size) * 100
				fmt.Printf("[%s | %s] Instant: %s/s, Average: %s/s, Loading: %.2f %% \n", remoteAddr, fileInfo.Filename, formatSpeed(inst), formatSpeed(avg), loading)
				lastPrint = tickTime
			case <-done:
				return
			}
		}
	}(conn.RemoteAddr().String(), fileInfo)

	buf := make([]byte, buffSize)
	var receivedBytes int64 = 0
	for receivedBytes < fileInfo.Size {
		remains := fileInfo.Size - receivedBytes
		nToRead := buffSize
		if int64(nToRead) > remains {
			nToRead = int(remains)
		}

		n, readErr := reader.Read(buf[:nToRead])
		if n > 0 {
			written := 0
			for written < n {
				wn, writeErr := file.Write(buf[written:n])
				if writeErr != nil {
					fmt.Printf("Error writing to file: %v\n", writeErr)
					_, err = conn.Write([]byte("Error failed to write file\n"))
					if err != nil {
						fmt.Println("Error writing to connection:", err.Error())
					}
					close(done)
					err = file.Close()
					if err != nil {
						fmt.Println("Error close the file:", err.Error())
					}
					err = os.Remove(pathFile)
					if err != nil {
						fmt.Println("Error remove the file:", err.Error())
					}
					return
				}
				written += wn
			}
			atomic.AddInt64(&totalBytes, int64(n))
			atomic.AddInt64(&bytesSinceLast, int64(n))
			receivedBytes += int64(n)
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			fmt.Printf("Error receiving file data: %v\n", readErr)

			_, err = conn.Write([]byte("Error Failed to receive file data\n"))
			if err != nil {
				fmt.Println("Error writing to connection:", err.Error())
			}

			close(done)

			err = file.Close()
			if err != nil {
				fmt.Println("Error close the file:", err.Error())
			}

			err = os.Remove(pathFile)
			if err != nil {
				fmt.Println("Error remove the file:", err.Error())
			}
			return
		}
	}

	byteLast := atomic.SwapInt64(&bytesSinceLast, 0)
	total := atomic.LoadInt64(&totalBytes)

	avg := float64(total) / time.Since(startTime).Seconds()

	if time.Since(startTime) < 3*time.Second || byteLast > 0 {
		instantSinceLast := float64(byteLast) / time.Since(lastPrint).Seconds()
		fmt.Printf("[%s | %s] Instant(final): %s/s, Average(final): %s/s\n", conn.RemoteAddr().String(), fileInfo.Filename, formatSpeed(instantSinceLast), formatSpeed(avg))
	}

	close(done)

	if total == fileInfo.Size {
		_, err = conn.Write([]byte("STATUS OK\n"))
		if err != nil {
			fmt.Println("Error writing to connection:", err.Error())
		}

		fmt.Printf("File %s received successfully (%s)\n", fileInfo.Filename, formatSpeed(float64(total)))
	} else {
		_, err = conn.Write([]byte("STATUS FAIL\n"))
		if err != nil {
			fmt.Println("Error writing to connection:", err.Error())
		}

		fmt.Printf("File size mismatch: expected %d, received %d\n", fileInfo.Size, receivedBytes)

		err = file.Close()
		if err != nil {
			fmt.Println("Error close the file:", err.Error())
		}

		err = os.Remove(pathFile)
		if err != nil {
			fmt.Println("Error remove the file:", err.Error())
		}
	}
}
