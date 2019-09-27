package framework

import (
	"crypto/rsa"
	"io"

	"golang.org/x/crypto/ssh"
)

func NewSSHKey(rand io.Reader) (ssh.PublicKey, *ssh.ClientConfig, error) {
	// NOT(nan) could not get ed25519 keys to work... since it's just for testing
	// rsa should be fine.
	// _, private, err := ed25519.GenerateKey(rand)
	private, err := rsa.GenerateKey(rand, 2048)
	if err != nil {
		return nil, nil, err
	}

	signer, err := ssh.NewSignerFromKey(private)
	if err != nil {
		return nil, nil, err
	}

	config := &ssh.ClientConfig{
		User: "root",
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	return signer.PublicKey(), config, nil
}
