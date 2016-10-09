package cache

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"
)

const (
	mode_http = "http"
	mode_tcp  = "tcp"
	mode_udp  = "udp"
)

type Config struct {
	mode    string
	address string

	expiration int
}

func main() {
	c := Config{}
	c.initFlags()
	cache := newCacheWithJanitor(time.Duration(c.expiration)*time.Second, time.Duration(c.expiration/10)*time.Second)
	cache.Add("a", []byte("am"), 0) //TODO delete this line
	switch c.mode {
	case mode_http:
		err := http.ListenAndServe(c.address, nil)
		if err != nil {
			fmt.Printf("Error: %v", err)
		}
	case mode_tcp:
		ln, err := net.Listen("tcp", c.address)
		if err != nil {
			//TODO handle error
		}
		for {
			conn, err := ln.Accept()
			if err != nil {
				//TODO handle error
			}
			go handleTCPConnection(conn, cache)
		}

	case mode_udp:
		udpAdd, err := net.ResolveUDPAddr("", c.address)
		if err != nil {
			log.Fatalln("Could not resolve address: " + c.address)
		}
		lnu, err := net.ListenUDP("udp", udpAdd)
		if err != nil {
			//TODO handle error
		}
		for {
			buf := make([]byte, 8192) //TODO parametrize it 8k
			_, _, err := lnu.ReadFromUDP(buf)
			//TODO hande incoming message.

			if err != nil {
				log.Fatal(err)
			}
		}

	default:
		panic("Not implemented mode : " + c.mode)
	}

}

func (c *Config) initFlags() {
	flag.StringVar(&c.mode, "http", "http", "mode of cachec server: can be "+mode_http+" "+mode_tcp+" or "+mode_udp)
	flag.StringVar(&c.address, "bind", "", "optional options to set listening specific interface: <ip ro hostname>:<port>")
	flag.IntVar(&c.expiration, "expiration", 200, "expiration time in seconds")
	flag.Parse()

	if err := c.checkFlags(); err != nil {
		fmt.Errorf("Error: %v", err)
		flag.PrintDefaults()
	}
}

func (c *Config) checkFlags() error {
	if c.mode != mode_http || c.mode != mode_tcp || c.mode != mode_udp {
		fmt.Errorf("Wrong mode: %s", c.mode)
	}
	return nil
}

//TODO rebuild to get set command
func handleTCPConnection(conn net.Conn, cache *Cache) {
	// Make a buffer to hold incoming data.
	buf := make([]byte, 1024)
	// Read the incoming connection into the buffer.
	_, err := conn.Read(buf)
	if err != nil {
		fmt.Println("Error reading:", err.Error())
	}
	// Send a response back to person contacting us.
	conn.Write([]byte("Message received."))
	// Close the connection when you're done with it.
	conn.Close()
}
