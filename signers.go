package bunker

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"

	"github.com/yankeguo/ufx"
	"go.uber.org/zap"
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

func loadOrCreateSigner(log *zap.SugaredLogger, filename string, generator SSHPrivateKeyGenerator) (sgn ssh.Signer, err error) {
	var buf []byte
	if buf, err = os.ReadFile(filename); err == nil {
		if sgn, err = ssh.ParsePrivateKey(buf); err != nil {
			return
		}

		log.With("filename", filename).Info("signer loaded")
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

		log.With("filename", filename).Info("signer generated")
	}
	return
}

func CreateSigners(log *zap.SugaredLogger, dir DataDir) (signers *Signers, err error) {
	signers = &Signers{}

	for _, item := range []struct {
		output *[]ssh.Signer
		prefix string
	}{
		{
			output: &signers.Host,
			prefix: "ssh_host_",
		},
		{
			output: &signers.Client,
			prefix: "ssh_client_",
		},
	} {
		for kind, generator := range sshPrivateKeyGenerators {
			var sgn ssh.Signer
			if sgn, err = loadOrCreateSigner(log, filepath.Join(dir.String(), item.prefix+kind+"_key"), generator); err != nil {
				return
			}
			*item.output = append(*item.output, sgn)
		}
	}

	for _, sgn := range signers.Client {
		signers.AuthorizedKeys += string(ssh.MarshalAuthorizedKey(sgn.PublicKey()))
	}

	log.Info("\n------- Client Public Keys -------\n" + strings.TrimSpace(signers.AuthorizedKeys) + "\n----------------------------------")

	return
}

func InstallSignersToRouter(ur ufx.Router, signers *Signers) {
	ur.HandleFunc("/backend/authorized_keys", func(c ufx.Context) {
		c.Text(signers.AuthorizedKeys)
	})
}
