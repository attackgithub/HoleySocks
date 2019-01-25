package holeysocks

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"

	"github.com/armon/go-socks5"
	"github.com/gobuffalo/packr"
	"golang.org/x/crypto/ssh"
)

// Config is the variable holding necessary variables for
// establishing the reverse SSH connection
var Config = MainConfig{}

func init() {
	// unpack the configs and ssh keys from the binary
	// that were packed at compile-time
	box := packr.NewBox("../../configs")
	configBytes, err := box.Find("config.json")
	if err != nil {
		panic(err)
	}

	if err := json.Unmarshal(configBytes, &Config); err != nil {
		panic(err)
	}

	privKeyBytes, err := box.Find("id_ed25519")
	if err != nil {
		panic(err)
	}
	Config.SSH.setKey(privKeyBytes)
}

// From https://sosedoff.com/2015/05/25/ssh-port-forwarding-with-go.html
// Handle local client connections and tunnel data to the remote server
// Will use io.Copy - http://golang.org/pkg/io/#Copy
func handleClient(client net.Conn, remote net.Conn) {
	defer client.Close()
	chDone := make(chan bool)

	// Start remote -> local data transfer
	go func() {
		_, err := io.Copy(client, remote)
		if err != nil {
			log.Fatalln(fmt.Sprintf("error while copy remote->local: %s", err))
		}
		chDone <- true
	}()

	// Start local -> remote data transfer
	go func() {
		_, err := io.Copy(remote, client)
		if err != nil {
			log.Fatalln(fmt.Sprintf("error while copy local->remote: %s", err))
		}
		chDone <- true
	}()

	<-chDone
}

// ForwardService implements reverse port forwarding similar to the -R flag
// in openssh-client. Configuration is done in the /configs/config.json file.
// NOTE The generated keys and config.json data are embedded in the binary so
// take the appropriate precautions when setting up the ssh server user.
func ForwardService() {
	sshClientConf := &ssh.ClientConfig{
		User:            Config.SSH.Username,
		Auth:            Config.SSH.PrivKey,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	// Connect to SSH server
	serverConn, err := ssh.Dial("tcp", Config.SSH.connectionString(), sshClientConf)
	if err != nil {
		log.Fatalln(fmt.Sprintf("SSH Conn Failed: %s", err))
	}

	// Publish the designated local port to the configured port on the remote SSH server
	remoteListener, err := serverConn.Listen("tcp", Config.Socks.Remote)
	if err != nil {
		log.Fatalln(fmt.Sprintf("Reverse port-forward failed : %s", err))
	}
	defer remoteListener.Close()

	// Handle incoming requests from the remote tunnel
	for {
		// Grab a handle to the pre-configured local port that will be sent to the remote
		// SSH server
		local, err := net.Dial("tcp", Config.Socks.Local)
		if err != nil {
			log.Fatalln(fmt.Sprintf("Unable to start local listen: %s", err))
		}

		// Grab a handle on the remote port
		remote, err := remoteListener.Accept()
		if err != nil {
			log.Fatalln(fmt.Sprintf("Unable to accept remote traffic locally: %s", err))
		}

		// Swap IO from the local and remote hanles
		handleClient(remote, local)
	}

}

// DarnSocks creates a new SOCKS5 server at the provided ports and
// remote-forwards the port to another machine over SSH
func DarnSocks() {
	// Create a SOCKS5 server
	conf := &socks5.Config{}
	server, err := socks5.New(conf)
	if err != nil {
		panic(err)
	}

	// Create SOCKS5 proxy on localhost
	server.ListenAndServe("tcp", Config.Socks.Local)
}
