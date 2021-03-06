/*
Laboratorio 6 - Web Proxy com Cache
Leandro Alves Vianna
Rafael de Castro Silva
*/

package main

import "fmt"
import "os"
import "os/signal"
import "github.com/leandrovianna/gowebproxy/info"
import "github.com/leandrovianna/gowebproxy/proxy"

func main() {
	const webProxyPort = 54321
	const infoPort = 54322

	stats := make(chan info.Stats)

	// inicia o servidor web proxy na porta 54321
	go proxy.ProxyWebServer(webProxyPort, stats)

	// inicia o servidor de infos na porta 54322
	go info.InfoServer(infoPort, stats)

	// capturar Ctrl-C (Interrupt Signal)
	// para encerrar o programa

	interruptChan := make(chan os.Signal, 1)
	signal.Notify(interruptChan, os.Interrupt)

	// bloqueia para manter as goroutines executando
	<-interruptChan
	fmt.Println("Server terminated.")
}
