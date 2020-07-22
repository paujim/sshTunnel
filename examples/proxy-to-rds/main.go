package main

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/denisenkom/go-mssqldb"
	"github.com/paujim/sshTunnel/pkg/tunnel"
)

func main() {

	local := &tunnel.Endpoint{
		Host: "localhost",
		Port: 4000,
	}
	proxyServer := &tunnel.Endpoint{
		Host: "ec2.address.url",
		Port: 22,
	}
	remoteServer := &tunnel.Endpoint{
		Host: "rds.address.url",
		Port: 1433,
	}
	dbName := "master"
	dbUsername := "test"
	dbPassword := "some password"

	query := "SELECT * FROM Samples"

	tn := tunnel.CreateTunnel(local, proxyServer, "ec2_user", "keyfile.pem", remoteServer)

	go tn.Start()
	time.Sleep(1 * time.Second)
	defer tn.Stop()
	connString := fmt.Sprintf("Server=%s;Port=%d;Database=%s;User Id=%s;password=%s; Connection Timeout=%v", local.Host, local.Port, dbName, dbUsername, dbPassword, 600)

	db, err := sql.Open("mssql", connString)
	if err != nil {
		log.Println(err.Error())
		panic(err)
	}
	defer db.Close()
	_, err = db.Exec(query)
	if err != nil {
		log.Println(err.Error())
		panic(err)
	}

	return
}
