package main

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/pem"
	"errors"
	"log"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/yankeguo/ufx"
	"go.uber.org/fx"
	"golang.org/x/crypto/ed25519"
	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"
)

const (
	sshHostKeyFileRSA     = "ssh_host_rsa_key"
	sshHostKeyFileECDSA   = "ssh_host_ecdsa_key"
	sshHostKeyFileEd25519 = "ssh_host_ed25519_key"
)

type SSHServerParams struct {
	Listen string `json:"listen" default:":8022" validate:"required"`
}

func createSSHServerParams(c ufx.Conf) (p SSHServerParams, err error) {
	err = c.Bind(&p, "ssh_server")
	return
}

type SSHServer struct {
	DataDir  string
	Params   SSHServerParams
	Database *gorm.DB

	listener    *net.TCPListener
	hostSigners []ssh.Signer
}

type SSHServerOptions struct {
	fx.In
	fx.Lifecycle

	Params   SSHServerParams
	DataDir  DataDir
	Database *gorm.DB
}

func (s *SSHServer) ensureHostSigner(filename string, generator func() (key crypto.PrivateKey, err error)) (err error) {
	var (
		sgn ssh.Signer
		buf []byte
	)
	if buf, err = os.ReadFile(filename); err == nil {
		if sgn, err = ssh.ParsePrivateKey(buf); err != nil {
			return
		}
		log.Println("host signer loaded from:", filename)
		s.hostSigners = append(s.hostSigners, sgn)
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

		s.hostSigners = append(s.hostSigners, sgn)

		var block *pem.Block
		if block, err = ssh.MarshalPrivateKey(key, ""); err != nil {
			return
		}

		buf = pem.EncodeToMemory(block)
		if err = os.WriteFile(filename, buf, 0600); err != nil {
			return
		}

		log.Println("host signer generated to:", filename)
	}
	return
}

func (s *SSHServer) ensureHostSigners() (err error) {
	fileRSA := filepath.Join(s.DataDir, sshHostKeyFileRSA)

	if err = s.ensureHostSigner(fileRSA, func() (crypto.PrivateKey, error) {
		return rsa.GenerateKey(rand.Reader, 2048)
	}); err != nil {
		return
	}

	fileECDSA := filepath.Join(s.DataDir, sshHostKeyFileECDSA)

	if err = s.ensureHostSigner(fileECDSA, func() (crypto.PrivateKey, error) {
		return ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	}); err != nil {
		return
	}

	fileEd25519 := filepath.Join(s.DataDir, sshHostKeyFileEd25519)

	if err = s.ensureHostSigner(fileEd25519, func() (crypto.PrivateKey, error) {
		_, priv, err := ed25519.GenerateKey(rand.Reader)
		return priv, err
	}); err != nil {
		return
	}

	return
}

func (s *SSHServer) HandleServerConn(conn net.Conn) {
	defer conn.Close()

	var err error

	cfg := &ssh.ServerConfig{
		PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			return nil, nil
		},
		BannerCallback: func(conn ssh.ConnMetadata) string {
			return "bunker from github.com/yankeguo/bunker"
		},
	}

	for _, sgn := range s.hostSigners {
		cfg.AddHostKey(sgn)
	}

	var (
		sc    *ssh.ServerConn
		chNew <-chan ssh.NewChannel
		chReq <-chan *ssh.Request
	)

	if sc, chNew, chReq, err = ssh.NewServerConn(conn, cfg); err != nil {
		return
	}

	_ = sc
	_ = chNew
	_ = chReq
}

func (s *SSHServer) ListenAndServe() (err error) {
	if s.listener != nil {
		err = errors.New("listener is already initialized")
		return
	}

	var addr *net.TCPAddr
	if addr, err = net.ResolveTCPAddr("tcp", s.Params.Listen); err != nil {
		return
	}

	if s.listener, err = net.ListenTCP("tcp", addr); err != nil {
		return
	}
	defer s.listener.Close()

	for {
		var conn net.Conn
		if conn, err = s.listener.Accept(); err != nil {
			return
		}
		go s.HandleServerConn(conn)
	}
}

func (s *SSHServer) Shutdown(ctx context.Context) (err error) {
	l := s.listener
	if l == nil {
		return
	}
	err = l.Close()
	return
}

func createSSHServer(opts SSHServerOptions) (s *SSHServer, err error) {
	s = &SSHServer{
		DataDir: opts.DataDir.String(),
		Params:  opts.Params,
	}

	if err = s.ensureHostSigners(); err != nil {
		return
	}

	if opts.Lifecycle != nil {
		opts.Lifecycle.Append(fx.Hook{
			OnStart: func(ctx context.Context) error {
				chErr := make(chan error, 1)
				go func() {
					chErr <- s.ListenAndServe()
				}()
				select {
				case err := <-chErr:
					return err
				case <-ctx.Done():
					return s.Shutdown(ctx)
				case <-time.After(time.Second * 3):
					return nil
				}
			},
			OnStop: func(ctx context.Context) error {
				time.Sleep(time.Second * 3)
				return s.Shutdown(ctx)
			},
		})
	}
	return
}
