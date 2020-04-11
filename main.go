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
	"sync"
	"time"

	"errors"
)

const CHUNK_SIZE = 1000000
const READ_SIZE = 100000
const FLUSH_SIZE = 10000

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

func downloadChunk(url *URL.URL, from, to int, cb func([]byte)) {
	rangeVal := fmt.Sprintf("bytes=%d-%d", from, to)

	headerFields := map[string]string{
		"Range":      rangeVal,
		"Connection": "close",
		"User-Agent": "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:74.0) Gecko/20100101 Firefox/74.0",
	}
	header := makeHeader("GET", url.Host, url.Path, headerFields)

	port := url.Port()
	if port == "" {
		port = "80"
	}
	tcpAddr, err := net.ResolveTCPAddr("tcp4", url.Host+":"+port)
	if err != nil {
		fmt.Println("ResolveTCPAddr error")
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

	for {
		body := make([]byte, READ_SIZE, READ_SIZE)
		len, err := reader.Read(body)
		if err != nil {
			if err != io.EOF {
				fmt.Println("downloadChunk err: ", err)
			}
			break
		}

		cb(body[:len])
	}
}

func checkAcceptMultipartDownload(url *URL.URL) (int, error) {
	h := makeHeader(
		"HEAD",
		url.Host,
		url.Path,
		map[string]string{
			"Accept-Ranges": "bytes",
			"Connection":    "close",
			"User-Agent":    "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:74.0) Gecko/20100101 Firefox/74.0",
		})

	port := url.Port()
	if port == "" {
		port = "80"
	}
	tcpAddr, err := net.ResolveTCPAddr("tcp4", url.Host+":"+port)
	if err != nil {
		fmt.Println("ResolveTCPAddr error")

		return 0, err
	}

	conn, err := makeRequest(tcpAddr, h)
	if err != nil {
		fmt.Println(err)
	}
	reader := bufio.NewReader(conn)

	header := readHeader(reader)

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
		chunks[i-1] = chunks[i-1] + size - from + 1
	}

	return chunks
}
func downloadFilePart(id int, wg *sync.WaitGroup, url *URL.URL, output string, rangeFrom, rangeTo int) {
	defer wg.Done()
	outputPath := fmt.Sprintf("%s%d", output, id)
	tempFile, err := os.OpenFile(outputPath, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0664)
	if err != nil {
		fmt.Println(err)
		return
	}
	w := bufio.NewWriter(tempFile)
	totalSize := 0
	tt := 0
	start := time.Now()
	downloadChunk(url, rangeFrom, rangeTo, func(data []byte) {
		nn, err := w.Write(data)
		totalSize = totalSize + nn
		tt += nn
		if err != nil {
			fmt.Println(err)
		}
		if totalSize >= FLUSH_SIZE {
			err = w.Flush()
			if err != nil {
				fmt.Println(err)
			}
			totalSize = 0
		}
	})
	err = w.Flush()
	if err != nil {
		fmt.Println(err)
	}
	t := time.Now()
	elapsed := t.Sub(start)
	fmt.Printf("Part: %d \t\t Time: %f seconds \n", id, elapsed.Seconds())
}

func main() {
	fmt.Println("Download file as parts")

	// URL of download file
	targetFileUrl := "https://images.pexels.com/photos/853199/pexels-photo-853199.jpeg?cs=srgb&dl=aerial-view-of-seashore-near-large-grey-rocks-853199.jpg&fm=jpg"
	// Save the target file as
	output := "./wall_pager.jpg"

	url, err := URL.Parse(targetFileUrl)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("Path ", url.Path)
	fmt.Println("Host ", url.Host)

	// Get file info
	size, err := checkAcceptMultipartDownload(url)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Println("Size: ", size)

	// Calculate range to download
	parts := 5
	downloadRanges := calculateDownloadRange(size, parts)
	fmt.Println("Down load ranges: ", downloadRanges)

	var wg sync.WaitGroup
	for i := 0; i < parts; i++ {
		from := downloadRanges[i*2]
		to := downloadRanges[i*2+1]
		wg.Add(1)
		go func(i, from, to int) {
			downloadFilePart(i, &wg, url, output, from, to)
		}(i, from, to)
	}
	wg.Wait()

	// Merge file
	// TODO: Create binary merge
	// round1: 1+2=a, 2+3b, 3+4=c
	// round2: a+b=f, f+c=final
	f0, err := os.OpenFile(output, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0664)
	if err != nil {
		fmt.Println(err)
	}

	start := time.Now()
	for i := 0; i < parts; i++ {
		fileName := fmt.Sprintf("%s%d", output, i)
		f, err := os.Open(fileName)
		if err != nil {
			fmt.Println("Cant open file ")
			return
		}
		for {
			data := make([]byte, READ_SIZE, READ_SIZE)
			n, err := f.Read(data)

			if err != nil {
				if err != io.EOF {
					fmt.Println(err)
					return
				}
				break
			}

			f0.Write(data[:n])
		}
		err = os.Remove(fileName)
		if err != nil {
			fmt.Println("Can not remove file ", fileName)
		}

	}
	t := time.Now()
	fmt.Printf("Merge files.\t\t Time: %f seconds\n", t.Sub(start).Seconds())
	fmt.Println("Output ", output)
}
