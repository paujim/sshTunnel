package tunnel

import (
	"io"
	"io/ioutil"
	"log"
	"net"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

type TunelIface interface {
	Start() error
	Stop()
}

type Tunnel struct {
	localEndpoint   *Endpoint
	proxyEndpoint   *Endpoint
	remoteEndpoint  *Endpoint
	sshConfig       *ssh.ClientConfig
	closeConnection chan bool
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
		closeConnection: make(chan bool),
	}
}

func (t *Tunnel) Start() error {
	log.Printf("Tunnel started!\n")
	listener, err := net.Listen("tcp", t.localEndpoint.String())
	if err != nil {
		log.Printf("Unable to create local connection: %s\n", err)
		return err
	}
	defer listener.Close()

	conn, err := listener.Accept()
	if err != nil {
		log.Printf("Unbale to accept listener: %s\n", err)
		return err
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go t.forward(conn, t.sshConfig, &wg)
	wg.Wait()
	return nil
}

func (t *Tunnel) Stop() {
	log.Printf("Tunnel stoped!\n")
	t.closeConnection <- true
}

func (t *Tunnel) forward(localConn net.Conn, sshConfig *ssh.ClientConfig, wg *sync.WaitGroup) {
	proxyConn, err := ssh.Dial("tcp", t.proxyEndpoint.String(), sshConfig)
	if err != nil {
		log.Printf("Unable to open proxy connection: %s\n", err)
		return
	}

	remoteConn, err := proxyConn.Dial("tcp", t.remoteEndpoint.String())
	if err != nil {
		log.Printf("Unable to open remote connection: %s\n", err)
		return
	}

	copyConn := func(writer, reader net.Conn) {
		_, err := io.Copy(writer, reader)
		if err != nil {
			log.Printf("Error coping: %s\n", err)
		}
	}

	go copyConn(localConn, remoteConn)
	go copyConn(remoteConn, localConn)
	<-t.closeConnection
	localConn.Close()
	proxyConn.Close()
	remoteConn.Close()
	wg.Done()
}
