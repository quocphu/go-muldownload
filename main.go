package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	URL "net/url"
	"os"
	"strconv"
	"strings"

	"errors"
)

const CHUNK_SIZE = 1000000
const READ_SIZE = 100000

func makeHeader(method, host, endpoint string, headers map[string]string) string {
	var b strings.Builder
	b.WriteString(method)
	b.WriteString(" /")
	b.WriteString(endpoint)
	b.WriteString(" HTTP/1.1")
	b.WriteString("\n")
	b.WriteString("Host: ")
	b.WriteString(host)
	b.WriteString("\n")

	for k, v := range headers {
		b.WriteString(k)
		b.WriteString(": ")
		b.WriteString(v)
		b.WriteString("\n")
	}

	return b.String()
}

func makeRequest(tcpAddr *net.TCPAddr, header string) (net.Conn, error) {
	conn, err := net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		return nil, err
	}
	_, err = fmt.Fprintln(conn, header)
	return conn, err
}

func readHeader(reader *bufio.Reader) map[string]string {
	m := make(map[string]string)

	version, err := reader.ReadString(' ')
	if err != nil {
		fmt.Println(err)
		return nil
	}
	status, err := reader.ReadString(' ')
	if err != nil {
		fmt.Println(err)
		return nil
	}
	reason, err := reader.ReadString('\n')
	if err != nil {
		fmt.Println(err)
		return nil
	}
	m["vesion"] = strings.Trim(version, " ")
	m["status"] = strings.Trim(status, " ")
	m["reason"] = strings.Trim(reason, " ")

	for {
		header, err := reader.ReadString('\n')

		if err != nil {
			fmt.Println(err)
			return nil
		}

		if header == "\r\n" {
			break
		}

		// Parse header key: value
		headerFields := strings.Split(header, ":")
		val := strings.Replace(headerFields[1], "\r\n", "", 1)
		val = strings.Replace(val, " ", "", 1)
		m[headerFields[0]] = val
	}

	return m
}

func getSize(host, fileURL string) {

}

func downloadChunk(from, to int, host, file string, cb func([]byte)) {
	rangeVal := "bytes="
	if from > 0 {
		rangeVal += fmt.Sprintf("%d", from)
	}
	rangeVal += "-"
	if to > 0 {
		rangeVal += fmt.Sprintf("%d", to)
	}

	headerFields := map[string]string{
		"Range": rangeVal,
	}
	header := makeHeader("GET", host, file, headerFields)

	tcpAddr, err := net.ResolveTCPAddr("tcp4", host)
	if err != nil {
		fmt.Println(err)
	}

	conn, err := makeRequest(tcpAddr, header)
	if err != nil {
		fmt.Println(err)
	}

	reader := bufio.NewReader(conn)

	responeHeader := readHeader(reader)

	if responeHeader["status"] != "206" {
		fmt.Println("Header status", responeHeader["status"])
		return
	}

	size := to - from
	downloadSize := 0

	for {
		readSize := size - downloadSize
		if readSize > READ_SIZE {
			readSize = READ_SIZE
		}
		body := make([]byte, readSize, readSize)
		len, err := reader.Read(body)
		if err != nil {
			if err == io.EOF {
				fmt.Println("Done")
			} else {
				fmt.Println(err)
			}
			break
		}
		readSize += len
		if len > 0 {
			cb(body[:len])
		}
	}
}

func checkAcceptMultipartDownload(uri string) (int, error) {
	url, err := URL.Parse(uri)
	if err != nil {
		return 0, err
	}

	h := makeHeader(
		"HEAD",
		url.Host,
		url.Path,
		map[string]string{
			"Accept-Ranges": "bytes",
		})

	tcpAddr, err := net.ResolveTCPAddr("tcp4", url.Host)
	if err != nil {
		return 0, err
	}

	conn, err := makeRequest(tcpAddr, h)
	if err != nil {
		fmt.Println(err)
	}
	reader := bufio.NewReader(conn)

	header := readHeader(reader)
	// fmt.Println(header)

	if header["Accept-Ranges"] != "bytes" {
		return 0, errors.New("Server does not support multipart download")
	}

	size, err := strconv.Atoi(header["Content-Length"])
	return size, err
}

func calculateDownloadRange(size, part int) []int {
	var chunkSize = size / part
	chunks := make([]int, part*2, part*2)
	from := 0
	i := 0
	for ; i < part*2-1; i += 2 {
		chunks[i] = from
		chunks[i+1] = from + chunkSize - 1
		from += chunkSize
	}
	if from < size {
		chunks[i-1] = chunks[i-1] + size - from
	}
	return chunks
}

func main() {
	fmt.Println("hello")

	downloadRanges := calculateDownloadRange(21, 5)
	fmt.Println(downloadRanges)
	return

	host := "localhost:3000"
	file := "pdf24.pdf"
	// file = "test.txt"
	des := "http://localhost:3000/pdf24.pdf"

	url, err := URL.Parse(des)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("url host", url.Path)
	fmt.Println("url host", url.Host)

	// downloadSize := 1000000 // 1MB
	// readSize := 10
	fmt.Println("Host", host)
	fmt.Println("File", file)
	fmt.Println("---------------------")

	// h := makeHeader(
	// 	"HEAD",
	// 	host,
	// 	file,
	// 	map[string]string{
	// 		"Accept-Ranges": "bytes",
	// 	})

	// // fmt.Println("header \n", h)

	// tcpAddr, err := net.ResolveTCPAddr("tcp4", host)
	// if err != nil {
	// 	fmt.Println(err)
	// }

	// conn, err := makeRequest(tcpAddr, h)
	// if err != nil {
	// 	fmt.Println(err)
	// }
	// reader := bufio.NewReader(conn)

	// header := readHeader(reader)
	// // fmt.Println(header)

	// if header["Accept-Ranges"] != "bytes" {
	// 	fmt.Println("Server does not support multipart download")
	// 	os.Exit(0)
	// }

	// size, err := strconv.Atoi(header["Content-Length"])
	size, err := checkAcceptMultipartDownload(des)

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Println("Download size: ", size)

	tempFile, err := os.OpenFile("./"+file, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0664)
	w := bufio.NewWriter(tempFile)
	totalSize := 0
	downloadChunk(0, size, host, file, func(data []byte) {
		fmt.Println("Dowload file ", len(data))
		totalSize = totalSize + len(data)
		// fmt.Println("Dowload file ", data)
		nn, err := w.Write(data)
		if err != nil {
			fmt.Println(err)
		}
		fmt.Println("Write ", nn)
		if totalSize > 100000 {
			err = w.Flush()
			if err != nil {
				fmt.Println(err)
			}

		}
	})

}
