package main

import (
	"context"
	"errors"
	"log"
	"net"
	"strings"
	"time"

	"github.com/yankeguo/bunker/model"
	"github.com/yankeguo/bunker/model/dao"
	"github.com/yankeguo/ufx"
	"go.uber.org/fx"
	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"
)

const (
	sshExtKeyError         = "bunker.error"
	sshExtKeyUserID        = "bunker.user_id"
	sshExtKeyServerID      = "bunker.server_id"
	sshExtKeyServerUser    = "bunker.server_user"
	sshExtKeyServerName    = "bunker.server_name"
	sshExtKeyServerAddress = "bunker.server_address"
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
	Signers  *Signers

	listener *net.TCPListener
}

type SSHServerOptions struct {
	fx.In
	fx.Lifecycle

	Params   SSHServerParams
	DataDir  DataDir
	Database *gorm.DB
	Signers  *Signers
}

func (s *SSHServer) createServerConfig() *ssh.ServerConfig {
	cfg := &ssh.ServerConfig{
		AuthLogCallback: func(conn ssh.ConnMetadata, method string, err error) {
			log.Println("auth:", conn.RemoteAddr(), conn.User(), method, err)
		},
		PublicKeyCallback: func(conn ssh.ConnMetadata, _key ssh.PublicKey) (perm *ssh.Permissions, err error) {
			db := dao.Use(s.Database)

			// find user key and user
			var key *model.Key
			if key, err = db.Key.Where(dao.Key.ID.Eq(
				strings.ToLower(ssh.FingerprintSHA256(_key)),
			)).Preload(dao.Key.User).First(); err != nil {
				return nil, err
			}

			if key.User.ID == "" {
				err = errors.New("key is not associated with any user")
				return
			}

			// find server
			splits := strings.Split(conn.User(), "@")
			if len(splits) != 2 {
				err = errors.New("invalid user format, should be user@server")
				return
			}

			var (
				serverUser = splits[0]
				serverName = splits[1]
			)

			var server *model.Server
			if server, err = db.Server.Where(dao.Server.ID.Eq(serverUser + "@" + serverName)).First(); err != nil {
				return
			}

			//TODO: VALIDATE GRANT

			perm = &ssh.Permissions{
				Extensions: map[string]string{
					sshExtKeyUserID:        key.User.ID,
					sshExtKeyServerID:      server.ID,
					sshExtKeyServerAddress: server.Address,
					sshExtKeyServerUser:    serverUser,
					sshExtKeyServerName:    serverName,
				},
			}
			return
		},
		BannerCallback: func(conn ssh.ConnMetadata) string {
			return "[bunker] "
		},
	}

	for _, sgn := range s.Signers.Host {
		cfg.AddHostKey(sgn)
	}

	return cfg
}

func (s *SSHServer) HandleServerConn(conn net.Conn) {
	defer conn.Close()

	var err error

	var (
		sc    *ssh.ServerConn
		chNew <-chan ssh.NewChannel
		chReq <-chan *ssh.Request
	)

	if sc, chNew, chReq, err = ssh.NewServerConn(conn, s.createServerConfig()); err != nil {
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
		Signers: opts.Signers,
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
