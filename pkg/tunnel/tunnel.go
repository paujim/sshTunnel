package tunnel

import (
	"io"
	"io/ioutil"
	"log"
	"net"
	"time"

	"golang.org/x/crypto/ssh"
)

// TunelIface ..
type TunelIface interface {
	Start()
	Stop()
	Error() error
}

// Tunnel ..
type Tunnel struct {
	localEndpoint  *Endpoint
	proxyEndpoint  *Endpoint
	remoteEndpoint *Endpoint
	sshConfig      *ssh.ClientConfig
	err            error
	conns          []net.Conn
	proxyConns     []*ssh.Client
	close          chan interface{}
	errCn          chan error
}

func publicKeyFile(file string) ssh.AuthMethod {
	buffer, err := ioutil.ReadFile(file)
	if err != nil {
		return nil
	}
	key, err := ssh.ParsePrivateKey(buffer)
	if err != nil {
		return nil
	}
	return ssh.PublicKeys(key)
}

func (t *Tunnel) forward(localConn net.Conn) {
	proxyConn, err := ssh.Dial("tcp", t.proxyEndpoint.String(), t.sshConfig)
	if err != nil {
		log.Printf("Error connecting to proxy: %s\n", err)
		t.errCn <- err
		return
	}
	log.Printf("connected to %s (1 of 2)\n", t.proxyEndpoint.String())
	remoteConn, err := proxyConn.Dial("tcp", t.remoteEndpoint.String())
	if err != nil {
		log.Printf("Error connecting to remote: %s\n", err)
		t.errCn <- err
		return
	}
	t.conns = append(t.conns, remoteConn)
	t.proxyConns = append(t.proxyConns, proxyConn)
	log.Printf("connected to %s (2 of 2)\n", t.remoteEndpoint.String())
	copyConn := func(writer, reader net.Conn) {
		_, err := io.Copy(writer, reader)
		if err != nil {
			log.Printf("io.Copy error: %s", err)
		}
	}
	go copyConn(localConn, remoteConn)
	go copyConn(remoteConn, localConn)

	return
}

// Start ..
func (t *Tunnel) Start() {
	log.Println("Start Tunnel")
	listener, err := net.Listen("tcp", t.localEndpoint.String())
	if err != nil {
		log.Println("Unable to create local connection")
		return
	}
	defer listener.Close()

	log.Println("Listening for connections")
	conn, err := listener.Accept()
	if err != nil {
		log.Printf("Error accepting connections: %s\n", err)
		return
	}
	t.conns = append(t.conns, conn)
	log.Println("Accepted connection")
	go t.forward(conn)
	select {
	case <-t.close:
		log.Println("Closing Signal")
	case err = <-t.errCn:
		t.err = err
		log.Println("Error Signal")
	}

	var total int
	total = len(t.conns)
	for i, conn := range t.conns {
		log.Printf("Closing [net.Conn] connections (%d of %d)", i+1, total)
		err := conn.Close()
		if err != nil {
			log.Printf(err.Error())
		}
	}
	total = len(t.proxyConns)
	for i, conn := range t.proxyConns {
		log.Printf("Closing [ssh.Client] connections (%d of %d)", i+1, total)
		err := conn.Close()
		if err != nil {
			log.Printf(err.Error())
		}
	}

	log.Printf("Tunnel closed")
	return
}

// Stop ..
func (t *Tunnel) Stop() {
	log.Println("Stop Tunnel")
	if t.err == nil {
		t.close <- struct{}{}
	}
	return
}

// Error ..
func (t *Tunnel) Error() error {
	return t.err
}

// CreateTunnel ..
func CreateTunnel(local *Endpoint, proxy *Endpoint, proxyUser, proxyKeyFile string, remote *Endpoint) TunelIface {
	return &Tunnel{
		localEndpoint:  local,
		proxyEndpoint:  proxy,
		remoteEndpoint: remote,
		sshConfig: &ssh.ClientConfig{
			User: proxyUser,
			Auth: []ssh.AuthMethod{
				publicKeyFile(proxyKeyFile),
			},
			HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
				return nil
			},
			Timeout: 3 * time.Second,
		},
		close: make(chan interface{}),
		errCn: make(chan error),
	}
}
