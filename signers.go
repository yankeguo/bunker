package main

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/pem"
	"log"
	"os"
	"path/filepath"

	"github.com/yankeguo/ufx"
	"golang.org/x/crypto/ssh"
)

type SSHPrivateKeyGenerator = func() (key crypto.PrivateKey, err error)

var (
	sshPrivateKeyGenerators = map[string]SSHPrivateKeyGenerator{
		"rsa": func() (crypto.PrivateKey, error) {
			return rsa.GenerateKey(rand.Reader, 2048)
		},
		"ecdsa": func() (crypto.PrivateKey, error) {
			return ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
		},
		"ed25519": func() (crypto.PrivateKey, error) {
			_, priv, err := ed25519.GenerateKey(rand.Reader)
			return priv, err
		},
	}
)

type Signers struct {
	Host   []ssh.Signer
	Client []ssh.Signer

	AuthorizedKeys string
}

func loadOrCreateSigner(filename string, generator SSHPrivateKeyGenerator) (sgn ssh.Signer, err error) {
	var buf []byte
	if buf, err = os.ReadFile(filename); err == nil {
		if sgn, err = ssh.ParsePrivateKey(buf); err != nil {
			return
		}
		log.Println("signer loaded from:", filename)
	} else {
		if !os.IsNotExist(err) {
			return
		}

		var key crypto.PrivateKey
		if key, err = generator(); err != nil {
			return
		}
		if sgn, err = ssh.NewSignerFromKey(key); err != nil {
			return
		}

		var block *pem.Block
		if block, err = ssh.MarshalPrivateKey(key, ""); err != nil {
			return
		}

		buf = pem.EncodeToMemory(block)
		if err = os.WriteFile(filename, buf, 0600); err != nil {
			return
		}

		log.Println("signer generated to:", filename)
	}
	return
}

func createSigners(dataDir DataDir) (signers *Signers, err error) {
	signers = &Signers{}

	for kind, generator := range sshPrivateKeyGenerators {
		var sgn ssh.Signer
		if sgn, err = loadOrCreateSigner(filepath.Join(dataDir.String(), "ssh_host_"+kind+"_key"), generator); err != nil {
			return
		}
		signers.Host = append(signers.Host, sgn)
	}

	for kind, generator := range sshPrivateKeyGenerators {
		var sgn ssh.Signer
		if sgn, err = loadOrCreateSigner(filepath.Join(dataDir.String(), "ssh_client_"+kind+"_key"), generator); err != nil {
			return
		}
		signers.Client = append(signers.Client, sgn)
	}

	for _, sgn := range signers.Client {
		signers.AuthorizedKeys += string(ssh.MarshalAuthorizedKey(sgn.PublicKey()))
	}

	return
}

func installAPIAuthorizedKeys(signers *Signers, ur ufx.Router) {
	ur.HandleFunc("/backend/authorized_keys", func(c ufx.Context) {
		c.Text(signers.AuthorizedKeys)
	})
}
