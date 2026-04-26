package target

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net"
	"os"
	"testing"

	"github.com/neel-03/configdrift/internal/config"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/ssh"
)

func generateTestKey() ([]byte, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	privateKeyPEM := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}
	return pem.EncodeToMemory(privateKeyPEM), nil
}

func setupMockSSHServer(t *testing.T, keyData []byte) (string, func()) {
	config := &ssh.ServerConfig{
		PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			return nil, nil
		},
	}

	signer, err := ssh.ParsePrivateKey(keyData)
	if err != nil {
		t.Fatalf("Failed to parse private key: %v", err)
	}
	config.AddHostKey(signer)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}

	go func() {
		for {
			nConn, err := listener.Accept()
			if err != nil {
				return
			}
			_, chans, reqs, err := ssh.NewServerConn(nConn, config)
			if err != nil {
				continue
			}
			go ssh.DiscardRequests(reqs)
			go func(chans <-chan ssh.NewChannel) {
				for newChannel := range chans {
					if newChannel.ChannelType() != "session" {
						_ = newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
						continue
					}
					channel, requests, err := newChannel.Accept()
					if err != nil {
						continue
					}
					go func(in <-chan *ssh.Request) {
						for req := range in {
							switch req.Type {
							case "exec":
								payload := string(req.Payload[4:]) // skip length prefix
								if payload == "cat /etc/config.yaml" {
									_, _ = channel.Write([]byte("key: value"))
									_ = req.Reply(true, nil)
									_, _ = channel.SendRequest("exit-status", false, []byte{0, 0, 0, 0})
									channel.Close()
								} else {
									_ = req.Reply(false, nil)
								}
							default:
								_ = req.Reply(false, nil)
							}
						}
					}(requests)
				}
			}(chans)
		}
	}()

	return listener.Addr().String(), func() { listener.Close() }
}

func TestSSHAdapter_Fetch(t *testing.T) {
	keyData, err := generateTestKey()
	assert.NoError(t, err)

	tmpKey, err := os.CreateTemp("", "ssh-key")
	assert.NoError(t, err)
	defer os.Remove(tmpKey.Name())
	_, err = tmpKey.Write(keyData)
	assert.NoError(t, err)
	tmpKey.Close()

	addr, cleanup := setupMockSSHServer(t, keyData)
	defer cleanup()

	host, portStr, _ := net.SplitHostPort(addr)
	var port int
	_, err = fmt.Sscanf(portStr, "%d", &port)
	assert.NoError(t, err)

	// Create known_hosts for the test server
	signer, _ := ssh.ParsePrivateKey(keyData)
	pubKey := signer.PublicKey()
	tmpKnownHosts, err := os.CreateTemp("", "known_hosts")
	assert.NoError(t, err)
	defer os.Remove(tmpKnownHosts.Name())
	
	marshaled := ssh.MarshalAuthorizedKey(pubKey)
	line := fmt.Sprintf("[%s]:%d %s", host, port, string(marshaled))
	_, err = tmpKnownHosts.WriteString(line)
	assert.NoError(t, err)
	tmpKnownHosts.Close()

	cfg := config.TargetConfig{
		Name:       "test-ssh",
		Type:       "ssh",
		Host:       host,
		Port:       port,
		User:       "testuser",
		Key:        tmpKey.Name(),
		KnownHosts: tmpKnownHosts.Name(),
		Path:       "/etc/config.yaml",
	}

	adapter := NewSSHAdapter(cfg)
	defer adapter.Close()

	data, err := adapter.Fetch(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, []byte("key: value"), data)
}
