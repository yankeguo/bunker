package bunker

import (
	"context"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/git-lfs/wildmatch"
	"github.com/yankeguo/bunker/model"
	"github.com/yankeguo/bunker/model/dao"
	"github.com/yankeguo/ufx"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"
)

const (
	sshExtKeyUserID        = "bunker.user_id"
	sshExtKeyServerID      = "bunker.server_id"
	sshExtKeyServerUser    = "bunker.server_user"
	sshExtKeyServerAddress = "bunker.server_address"
)

type SSHServer struct {
	dataDir  string
	listen   string
	db       *gorm.DB
	signers  *Signers
	loggers  *zap.SugaredLogger
	listener *net.TCPListener
}

type sshServerParams struct {
	Listen string `json:"listen" default:":8022" validate:"required"`
}

type SSHServerOptions struct {
	fx.In

	Lifecycle fx.Lifecycle
	Conf      ufx.Conf
	DataDir   DataDir
	DB        *gorm.DB
	Signers   *Signers
	Logger    *zap.SugaredLogger
}

func CreateSSHServer(opts SSHServerOptions) (s *SSHServer, err error) {
	var p sshServerParams

	if err = opts.Conf.Bind(&p, "ssh_server"); err != nil {
		return
	}

	s = &SSHServer{
		dataDir: opts.DataDir.String(),
		listen:  p.Listen,
		signers: opts.Signers,
		loggers: opts.Logger,
		db:      opts.DB,
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

func (s *SSHServer) AuthLogCallback(conn ssh.ConnMetadata, method string, err error) {
	s.loggers.With(
		"remote_addr", conn.RemoteAddr().String(),
		"method", method,
		"error", err,
	).Info("ssh auth")
}

func (s *SSHServer) PublicKeyCallback(conn ssh.ConnMetadata, _key ssh.PublicKey) (perm *ssh.Permissions, err error) {
	db := dao.Use(s.db)

	// find key and user
	var key *model.Key
	if key, err = db.Key.Where(db.Key.ID.Eq(
		ssh.FingerprintSHA256(_key),
	)).Preload(db.Key.User).First(); err != nil {
		return nil, err
	}

	if key.User.ID == "" {
		err = errors.New("key is not associated with any user")
		return
	}

	if key.User.IsBlocked {
		err = errors.New("user is blocked")
		return
	}

	// find server
	splits := strings.Split(conn.User(), "@")
	if len(splits) != 2 {
		err = errors.New("invalid user format, should be server_user@server_id")
		return
	}

	var (
		serverUser = splits[0]
		serverID   = splits[1]
	)

	var server *model.Server
	if server, err = db.Server.Where(db.Server.ID.Eq(serverID)).First(); err != nil {
		return
	}

	// find grants
	var grants = []*model.Grant{}
	if grants, err = db.Grant.Where(db.Grant.UserID.Eq(key.User.ID)).Find(); err != nil {
		return
	}

	var granted bool

	// check if user is granted
	for _, grant := range grants {
		var (
			mServerUser = wildmatch.NewWildmatch(grant.ServerUser, wildmatch.Basename, wildmatch.CaseFold)
			mServerID   = wildmatch.NewWildmatch(grant.ServerID, wildmatch.Basename, wildmatch.CaseFold)
		)

		if mServerUser.Match(serverUser) && mServerID.Match(serverID) {
			granted = true
			break
		}
	}

	if !granted {
		err = errors.New("no grant found")
		return
	}

	perm = &ssh.Permissions{
		Extensions: map[string]string{
			sshExtKeyUserID:        key.User.ID,
			sshExtKeyServerID:      server.ID,
			sshExtKeyServerAddress: server.Address,
			sshExtKeyServerUser:    serverUser,
		},
	}
	return
}

func (s *SSHServer) BannerCallback(conn ssh.ConnMetadata) string {
	return "[bunker] "
}

func (s *SSHServer) createServerConfig() *ssh.ServerConfig {
	cfg := &ssh.ServerConfig{
		AuthLogCallback:   s.AuthLogCallback,
		PublicKeyCallback: s.PublicKeyCallback,
		BannerCallback:    s.BannerCallback,
	}

	for _, sgn := range s.signers.Host {
		cfg.AddHostKey(sgn)
	}

	return cfg
}

func (s *SSHServer) HandleServerConn(conn net.Conn) {
	defer conn.Close()

	var err error

	var (
		userConn         *ssh.ServerConn
		chUserNewChannel <-chan ssh.NewChannel
		chUserRequest    <-chan *ssh.Request
	)

	if userConn, chUserNewChannel, chUserRequest, err = ssh.NewServerConn(conn, s.createServerConfig()); err != nil {
		return
	}
	defer userConn.Close()

	var (
		serverUser    = userConn.Permissions.Extensions[sshExtKeyServerUser]
		serverAddress = userConn.Permissions.Extensions[sshExtKeyServerAddress]
	)

	if _, port, _ := net.SplitHostPort(serverAddress); port == "" {
		serverAddress = net.JoinHostPort(serverAddress, "22")
	}

	log := s.loggers.With(
		"remote_addr", conn.RemoteAddr().String(),
		"server_user", serverUser,
		"server_address", serverAddress,
		"server_id", userConn.Permissions.Extensions[sshExtKeyServerID],
		"session_id", hex.EncodeToString(userConn.SessionID()),
	)

	var client *ssh.Client
	if client, err = ssh.Dial("tcp", serverAddress, &ssh.ClientConfig{
		User: serverUser,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(s.signers.Client...),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}); err != nil {
		log.With("error", err).Error("ssh dial")
		return
	}
	defer client.Close()

	PipeSSH(log, client, userConn, chUserNewChannel, chUserRequest)
}

func (s *SSHServer) ListenAndServe() (err error) {
	if s.listener != nil {
		err = errors.New("listener is already initialized")
		return
	}

	var addr *net.TCPAddr
	if addr, err = net.ResolveTCPAddr("tcp", s.listen); err != nil {
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

func PipeSSH(log *zap.SugaredLogger, target *ssh.Client, userConn *ssh.ServerConn, chUserNewChannel <-chan ssh.NewChannel, chUserRequest <-chan *ssh.Request) {
	// handle user request for new channel
	handleUserNewChannel := func(wg *sync.WaitGroup, userNewChannel ssh.NewChannel) {
		defer wg.Done()

		log := log.With("channel_type", userNewChannel.ChannelType())

		// create target channel and target request channel
		targetChannel, chTargetRequest, err1 := target.OpenChannel(userNewChannel.ChannelType(), userNewChannel.ExtraData())
		if err1 != nil {
			log.With("error", err1).Error("ssh open target channel")
			if errOpenFailed, ok := err1.(*ssh.OpenChannelError); ok {
				userNewChannel.Reject(errOpenFailed.Reason, errOpenFailed.Message)
			} else {
				userNewChannel.Reject(ssh.ConnectionFailed, err1.Error())
			}
			return
		}
		defer log.Info("channel end")
		defer targetChannel.Close()

		userChannel, chUserRequest, err1 := userNewChannel.Accept()
		if err1 != nil {
			log.With("error", err1).Error("ssh accept user channel")
			return
		}

		wg1 := &sync.WaitGroup{}

		wg1.Add(1)
		go func() {
			defer wg1.Done()
			defer log.Info("channel pipe end: from target")
			defer userChannel.Close()
			io.Copy(userChannel, targetChannel)
		}()

		wg1.Add(1)
		go func() {
			defer wg1.Done()
			defer log.Info("channel pipe end: from user")
			defer targetChannel.Close()
			io.Copy(targetChannel, userChannel)
		}()

		wg1.Add(1)
		go func() {
			defer wg1.Done()
			defer log.Info("channel request end: from target")
			for targetRequest := range chTargetRequest {
				ok, err2 := userChannel.SendRequest(targetRequest.Type, targetRequest.WantReply, targetRequest.Payload)
				if targetRequest.WantReply {
					targetRequest.Reply(ok, nil)
				}
				if err2 != nil {
					log.With("error", err2).Error("ssh send target request")
				}
			}
		}()

		wg1.Add(1)
		go func() {
			defer wg1.Done()
			defer log.Info("channel request end: from user")
			for userRequest := range chUserRequest {
				ok, err2 := targetChannel.SendRequest(userRequest.Type, userRequest.WantReply, userRequest.Payload)
				if userRequest.WantReply {
					userRequest.Reply(ok, nil)
				}
				if err2 != nil {
					log.With("error", err2).Error("ssh send user request")
				}
			}
		}()

		wg1.Wait()
	}

	handleUserRequest := func(wg *sync.WaitGroup, userRequest *ssh.Request) {
		defer wg.Done()

		log.With("request_type", userRequest.Type).Info("user global request")

		ok, buf, err1 := target.SendRequest(userRequest.Type, userRequest.WantReply, userRequest.Payload)
		if userRequest.WantReply {
			userRequest.Reply(ok, buf)
		}
		if err1 != nil {
			log.With("error", err1).Error("ssh send global user request")
		}
	}

	wg := &sync.WaitGroup{}

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer log.Info("user new chan end")

		wg1 := &sync.WaitGroup{}
		for userNewChannel := range chUserNewChannel {
			wg1.Add(1)
			go handleUserNewChannel(wg1, userNewChannel)
		}
		wg1.Wait()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer log.Info("user request end")

		wg1 := &sync.WaitGroup{}
		for userRequest := range chUserRequest {
			wg1.Add(1)
			go handleUserRequest(wg1, userRequest)
		}
		wg1.Wait()
	}()

	wg.Wait()
}
