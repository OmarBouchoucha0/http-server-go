package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
)

type HttpRequest struct {
	Method    string
	Path      string
	version   string
	Host      string
	UserAgent string
	body      []byte
}

func readData(conn net.Conn) []byte {
	fmt.Println("in readData")
	packet := make([]byte, 1024)
	n, err := conn.Read(packet)
	fmt.Println("finished reading data")
	if err != nil {
		fmt.Println("Faile to read data")
	}
	return packet[:n]
}

func respondOK(conn net.Conn, httpRequest HttpRequest) {
	content := strings.TrimPrefix(httpRequest.Path, "/echo/")
	contentLen := fmt.Sprint(len(content))
	fmt.Println(content)
	responseHeader := "HTTP/1.1 200 OK\r\n"
	contentType := "Content-Type: text/plain\r\n"
	contentLength := "Content-Length: " + contentLen + "\r\n"
	body := "\r\n" + content + "\r\n\r\n"
	response := responseHeader + contentType + contentLength + body
	fmt.Println(response)
	_, err := conn.Write([]byte(response))
	if err != nil {
		fmt.Println("Error writing response:", err)
		return
	}
	fmt.Println("200 OK")
}

func respondAgentOk(conn net.Conn, httpRequest HttpRequest) {
	fmt.Println(httpRequest.Path)
	content := strings.TrimPrefix(httpRequest.UserAgent, "User-Agent: ")
	contentLen := fmt.Sprint(len(content))
	responseHeader := "HTTP/1.1 200 OK\r\n"
	contentType := "Content-Type: text/plain\r\n"
	contentLength := "Content-Length: " + contentLen + "\r\n"
	body := "\r\n" + content + "\r\n\r\n"
	response := responseHeader + contentType + contentLength + body
	_, err := conn.Write([]byte(response))
	if err != nil {
		fmt.Println("Error writing response:", err)
		return
	}
}
func respondPostFileOk(conn net.Conn, file string, directory *string, content []byte) {
	lines := strings.Split(string(content), "\r\n\r\n")
	headerLength := len(lines[0]) + 4
	path := *directory + file
	err := os.WriteFile(path, content[headerLength:], 0644)
	if err != nil {
		fmt.Println("Error Writing file:", err)
		return
	}

	contentLen := fmt.Sprint(len(content))
	responseHeader := "HTTP/1.1 201 OK\r\n"
	contentType := "Content-Type: application/octet-stream\r\n"
	contentLength := "Content-Length: " + contentLen + "\r\n"
	body := "\r\n\r\n"
	response := responseHeader + contentType + contentLength + body
	fmt.Printf("response : \n%v\n", content[headerLength:])
	n, err := conn.Write([]byte(response))
	if err != nil || n < 0 {
		fmt.Println("Error writing response:", err)
		return
	}
}

func respondGetFileOk(conn net.Conn, file string, directory *string) {
	path := *directory + file
	content, err := os.ReadFile(path)
	if err != nil {
		fmt.Println("Error reading file:", err)
		return
	}
	contentLen := fmt.Sprint(len(content))
	responseHeader := "HTTP/1.1 200 OK\r\n"
	contentType := "Content-Type: application/octet-stream\r\n"
	contentLength := "Content-Length: " + contentLen + "\r\n"
	body := "\r\n" + string(content) + "\r\n\r\n"
	response := responseHeader + contentType + contentLength + body
	fmt.Printf("response : \n%v\n", response)
	n, err := conn.Write([]byte(response))
	if err != nil || n < 0 {
		fmt.Println("Error writing response:", err)
		return
	}
	fmt.Println("200 OK")
}

func respondNotFound(conn net.Conn) {
	response := "HTTP/1.1 404 Not Found\r\n\r\n"
	_, err := conn.Write([]byte(response))
	if err != nil {
		fmt.Println("Error writing response:", err)
		return
	}
	fmt.Println("404 Not Found")
}

func parseHttpRequest(packet []byte) HttpRequest {
	req := string(packet)
	splitReq := strings.Split(req, "\r\n")
	firstLine := strings.Fields(splitReq[0])
	headerLength := len(splitReq[0]) + len(splitReq[1]) + len(splitReq[2])
	HttpRequest := HttpRequest{
		firstLine[0],
		firstLine[1],
		firstLine[2],
		splitReq[1],
		splitReq[2],
		packet[headerLength:],
	}
	return HttpRequest
}

func isFileInPath(targetFile string, files []os.DirEntry) bool {
	for _, file := range files {
		if file.Name() == targetFile {
			return true
		}
	}
	return false
}

func handleConnection(conn net.Conn, directory *string, wg *sync.WaitGroup) {

	fmt.Println("started reading data")
	packet := readData(conn)
	fmt.Println("fineshed reading data")
	HttpRequest := parseHttpRequest(packet)
	fmt.Printf("httpRequest : %v\n", HttpRequest)
	if HttpRequest.Path == "/" {
		response := "HTTP/1.1 200 OK\r\n\r\n"
		conn.Write([]byte(response))
	} else if strings.HasPrefix(HttpRequest.Path, "/echo/") {
		respondOK(conn, HttpRequest)
	} else if strings.HasPrefix(HttpRequest.Path, "/user-agent") {
		respondAgentOk(conn, HttpRequest)
	} else if strings.HasPrefix(HttpRequest.Path, "/files/") {
		files, err := os.ReadDir(*directory)
		if err != nil {
			fmt.Println("Error reading directory:", err)
			os.Exit(1)
		}
		file := strings.TrimPrefix(HttpRequest.Path, "/files/")

		if HttpRequest.Method == "POST" {
			content := HttpRequest.body
			respondPostFileOk(conn, file, directory, content)
		} else if HttpRequest.Method == "GET" {
			if isFileInPath(file, files) {
				respondGetFileOk(conn, file, directory)
			} else {
				respondNotFound(conn)
			}
		} else {
			respondNotFound(conn)
		}
	} else {
		respondNotFound(conn)
	}
	conn.Close()
	wg.Done()
}

func main() {
	fmt.Println("Logs from your program will appear here!")

	directory := flag.String("directory", ".", "Directory to serve files from")
	flag.Parse()

	l, err := net.Listen("tcp", "0.0.0.0:4221")
	if err != nil {
		fmt.Println("Failed to bind to port 4221")
		os.Exit(1)
	}
	defer l.Close()

	var wg sync.WaitGroup
	defer wg.Wait()
	for {
		wg.Add(1)
		conn, err := l.Accept()
		if err != nil {
			fmt.Println("Error accepting connection: ", err.Error())
			os.Exit(1)
		}
		go handleConnection(conn, directory, &wg)
	}
}
