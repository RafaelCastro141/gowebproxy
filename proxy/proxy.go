package proxy

import (
	"bufio"
	"fmt"
	"gowebproxy/cache"
	"gowebproxy/info"
	"gowebproxy/log"
	"gowebproxy/parser"
	"net"
	"strconv"
	"time"
)

func ProxyWebServer(port int, statsChan chan info.Stats) {
	host := ":" + strconv.Itoa(port)
	// cria socket tcp na porta port
	listen, err := net.Listen("tcp", host)

	if err != nil {
		log.PrintError(err)
		return
	}

	defer listen.Close()

	fmt.Printf("Web Proxy listening in port %d\n", port)

	// enviando informação de inicio de execução
	statsChan <- info.Stats{StartTime: time.Now()}

	var connCount = 0
	var cache cache.Cache

	for {
		// loop infinito esperando por conexoes
		conn, err := listen.Accept()

		if err != nil {
			// se ocorrer um erro, imprimir e esperar por novas conexoes
			log.PrintError(err)
		} else {
			// se nao houver erro, tratar conexao em outra goroutine
			go handler(connCount, conn, statsChan, &cache)
			connCount++
		}
	}
}

func handler(connId int, conn net.Conn, statsChan chan info.Stats, cache *cache.Cache) {
	defer conn.Close()

	statsChan <- info.Stats{ActiveConn: 1}

	clientHostAddr := conn.RemoteAddr().String()

	log.LogInfo(connId, "Connection from %s\n", clientHostAddr)

	// criando leitor de mensagens da conexao
	var reader = bufio.NewReader(conn)
	var writer = bufio.NewWriter(conn)

	var serverConn net.Conn
	var serverReader *bufio.Reader
	var serverWriter *bufio.Writer

	// loop de leitura de mensagens
OUTERLOOP:
	for {
		request, err := parser.NewHttpRequest(reader)

		if err != nil {
			log.LogInfo(connId, "Error in parse HTTP request: %v\n", err)
			break OUTERLOOP
		}

		host, ok := request.Headers["Host"]

		if ok == false {
			log.LogInfo(connId, "Host do not exist, get URI %s\n", request.URI)
			break OUTERLOOP
		}

		// verificar se cache para esta request existe
		response, ok := cache.Get(request.Method, request.URI)

		if ok == false {
			// nao foi encontrado cache
			// cria conexao com servidor
			serverConn, err = net.Dial("tcp", host+":80")

			if err != nil {
				log.LogInfo(connId, "Error when trying to connect to host server %s: %v\n", host, err)
				break OUTERLOOP
			}

			serverReader = bufio.NewReader(serverConn)
			serverWriter = bufio.NewWriter(serverConn)

			// faz requisicao a host server
			log.LogInfo(connId, "Requesting to host %s the resource %s\n", host, request.URI)

			// enviando requisicao http para o host server
			parser.WriteHttpRequest(serverWriter, &request)

			log.LogInfo(connId, "Processing host %s http response\n", host)

			response, err = parser.NewHttpResponse(serverReader)

			if err != nil {
				log.LogInfo(connId, "(Host: %s) Error on parse HTTP response: %v\n", host, err)
				break OUTERLOOP
			}

			// por enquanto, sempre fechar conexao com servidor
			serverConn.Close()
			serverConn = nil

			// guardando na cache a resposta
			cache.Set(request.Method, request.URI, response)
		}

		// enviando corpo de resposta http do servidor (ou cache disp.) para o cliente do proxy
		parser.WriteHttpResponse(writer, &response)

		contentLengthStr, ok := response.Headers["Content-Length"]
		contentLength := 0
		if ok {
			contentLength, err = strconv.Atoi(contentLengthStr)
			if err != nil {
				log.LogInfo(connId, "Error: Content-Length is not numeric.\n")
				contentLength = 0
			}
		}

		statsChan <- info.Stats{
			LastHostsVisited:    []string{host},
			LastResourceVisited: []info.Resource{{request.URI, contentLength}},
		}

		// decide se mantem conexao com cliente proxy
		if connValue, ok := response.Headers["Connection"]; ok && connValue == "close" {
			break OUTERLOOP
		} else {
			log.LogInfo(connId, "Keeping connection with %s\n", clientHostAddr)
		}
	}

	log.LogInfo(connId, "Closing connection with %s\n", clientHostAddr)

	statsChan <- info.Stats{ActiveConn: -1}
}
