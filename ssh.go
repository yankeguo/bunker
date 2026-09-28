package bunker

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/yankeguo/bunker/model"
	"github.com/yankeguo/bunker/model/dao"
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
	listen  string
	db      *gorm.DB
	signers *Signers
	log     *slog.Logger

	mu       sync.Mutex
	listener *net.TCPListener
}

func NewSSHServer(cfg Config, db *gorm.DB, signers *Signers, log *slog.Logger) *SSHServer {
	return &SSHServer{
		listen:  cfg.SSHServer.Listen,
		signers: signers,
		log:     log,
		db:      db,
	}
}

func (s *SSHServer) AuthLogCallback(conn ssh.ConnMetadata, method string, err error) {
	log := s.log.With(
		"remote_addr", conn.RemoteAddr().String(),
		"user", conn.User(),
		"method", method,
	)

	if err != nil {
		log.With("error", err).Info("ssh auth failed")
	} else {
		log.Info("ssh auth succeeded")
	}
}

func (s *SSHServer) PublicKeyCallback(conn ssh.ConnMetadata, _key ssh.PublicKey) (perm *ssh.Permissions, err error) {
	db := dao.Use(s.db)

	// find key and user
	var key *model.Key
	if key, err = db.Key.Where(db.Key.ID.Eq(
		ssh.FingerprintSHA256(_key),
	)).Preload(db.Key.User).First(); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			err = errors.New("unknown public key")
		}
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

	var (
		serverUser string
		serverID   string
	)
	if serverUser, serverID, err = parseSSHTarget(conn.User()); err != nil {
		return
	}

	var server *model.Server
	if server, err = db.Server.Where(db.Server.ID.Eq(serverID)).First(); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			err = fmt.Errorf("unknown server %q", serverID)
		}
		return
	}

	// find grants
	var grants = []*model.Grant{}
	if grants, err = db.Grant.Where(db.Grant.UserID.Eq(key.User.ID)).Find(); err != nil {
		return
	}

	var granted bool

	for _, grant := range grants {
		if grantMatches(grant, serverUser, serverID) {
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
	return fmt.Sprintf(
		"bunker: welcome user %s from %s, session: %s\n",
		conn.User(),
		conn.RemoteAddr().String(),
		hex.EncodeToString(conn.SessionID()),
	)
}

// securePublicKeyAuthAlgos are the user authentication algorithms bunker accepts.
// ssh-rsa (SHA-1) and ssh-dss are omitted.
var securePublicKeyAuthAlgos = []string{
	ssh.KeyAlgoED25519,
	ssh.KeyAlgoSKED25519,
	ssh.KeyAlgoSKECDSA256,
	ssh.KeyAlgoECDSA256,
	ssh.KeyAlgoECDSA384,
	ssh.KeyAlgoECDSA521,
	ssh.KeyAlgoRSASHA256,
	ssh.KeyAlgoRSASHA512,
}

// secureHostKeyAlgos are the host key algorithms bunker will accept from a target.
var secureHostKeyAlgos = []string{
	ssh.KeyAlgoED25519,
	ssh.KeyAlgoECDSA256,
	ssh.KeyAlgoECDSA384,
	ssh.KeyAlgoECDSA521,
	ssh.KeyAlgoRSASHA256,
	ssh.KeyAlgoRSASHA512,
}

const maxSSHChannels = 32

func (s *SSHServer) createServerConfig() *ssh.ServerConfig {
	cfg := &ssh.ServerConfig{
		AuthLogCallback:         s.AuthLogCallback,
		PublicKeyCallback:       s.PublicKeyCallback,
		BannerCallback:          s.BannerCallback,
		MaxAuthTries:            6,
		PublicKeyAuthAlgorithms: securePublicKeyAuthAlgos,
	}

	for _, sgn := range s.signers.Host {
		cfg.AddHostKey(sgn)
	}

	return cfg
}

const (
	sshDialTimeout      = time.Second * 15
	sshHandshakeTimeout = time.Second * 15
)

func dialSSHServer(address string, config *ssh.ClientConfig) (client *ssh.Client, err error) {
	var conn net.Conn
	if conn, err = (&net.Dialer{Timeout: sshDialTimeout}).Dial("tcp", address); err != nil {
		return
	}

	_ = conn.SetDeadline(time.Now().Add(sshHandshakeTimeout))

	var (
		clientConn ssh.Conn
		channels   <-chan ssh.NewChannel
		requests   <-chan *ssh.Request
	)

	if clientConn, channels, requests, err = ssh.NewClientConn(conn, address, config); err != nil {
		_ = conn.Close()
		return
	}

	_ = conn.SetDeadline(time.Time{})

	client = ssh.NewClient(clientConn, channels, requests)
	return
}

func (s *SSHServer) HandleServerConn(conn net.Conn) {
	defer conn.Close()

	// bound the handshake so a client cannot hold a goroutine open
	_ = conn.SetDeadline(time.Now().Add(sshHandshakeTimeout))

	userConn, chUserNewChannel, chUserRequest, err := ssh.NewServerConn(conn, s.createServerConfig())
	_ = conn.SetDeadline(time.Time{})
	if err != nil {
		return
	}
	defer userConn.Close()

	var (
		serverUser    = userConn.Permissions.Extensions[sshExtKeyServerUser]
		serverAddress = userConn.Permissions.Extensions[sshExtKeyServerAddress]
	)

	serverAddress = withDefaultSSHPort(serverAddress)

	log := s.log.With(
		"remote_addr", conn.RemoteAddr().String(),
		"server_user", serverUser,
		"server_address", serverAddress,
		"server_id", userConn.Permissions.Extensions[sshExtKeyServerID],
		"session_id", hex.EncodeToString(userConn.SessionID()),
	)

	var client *ssh.Client
	if client, err = dialSSHServer(serverAddress, &ssh.ClientConfig{
		User: serverUser,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(s.signers.Client...),
		},
		HostKeyAlgorithms: secureHostKeyAlgos,
		HostKeyCallback:   s.hostKeyCallback(userConn.Permissions.Extensions[sshExtKeyServerID]),
	}); err != nil {
		log.With("error", err).Error("ssh dial")

		// keep the user connection alive and reject everything until the
		// user disconnects, so the failure is delivered to the client
		var wg sync.WaitGroup

		wg.Add(1)
		go func() {
			defer wg.Done()
			for nc := range chUserNewChannel {
				nc.Reject(ssh.ConnectionFailed, err.Error())
			}
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			for req := range chUserRequest {
				if req.WantReply {
					req.Reply(false, nil)
				}
			}
		}()

		wg.Wait()
		return
	}
	defer client.Close()

	log.Info("ssh connection established")

	PipeSSH(log, client, userConn, chUserNewChannel, chUserRequest)
}

func (s *SSHServer) ListenAndServe() (err error) {
	var addr *net.TCPAddr
	if addr, err = net.ResolveTCPAddr("tcp", s.listen); err != nil {
		return
	}

	s.mu.Lock()

	if s.listener != nil {
		s.mu.Unlock()
		err = errors.New("listener is already initialized")
		return
	}

	var listener *net.TCPListener
	if listener, err = net.ListenTCP("tcp", addr); err != nil {
		s.mu.Unlock()
		return
	}

	s.listener = listener
	s.mu.Unlock()

	defer listener.Close()

	for {
		var conn net.Conn
		if conn, err = listener.Accept(); err != nil {
			if errors.Is(err, net.ErrClosed) {
				err = nil
			}
			return
		}
		go s.HandleServerConn(conn)
	}
}

// Shutdown stops accepting connections. Sessions already running keep going
// until the process exits. ctx is accepted so callers can share a timeout
// with the HTTP server; closing the listener does not wait on it.
func (s *SSHServer) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	listener := s.listener
	s.mu.Unlock()

	if listener == nil {
		return nil
	}

	err := listener.Close()
	if errors.Is(err, net.ErrClosed) {
		return nil
	}
	return err
}

// withDefaultSSHPort adds port 22 when address has none. Bracketed and bare
// IPv6 addresses are both accepted.
func withDefaultSSHPort(address string) string {
	if _, port, _ := net.SplitHostPort(address); port == "" {
		return net.JoinHostPort(address, "22")
	}
	return address
}

func PipeSSH(log *slog.Logger, target *ssh.Client, userConn *ssh.ServerConn, chUserNewChannel <-chan ssh.NewChannel, chUserRequest <-chan *ssh.Request) {
	// handle user request for new channel
	handleUserNewChannel := func(wg *sync.WaitGroup, userNewChannel ssh.NewChannel) {
		defer wg.Done()

		log := log.With("channel_type", userNewChannel.ChannelType())

		// create target channel and target request channel
		targetChannel, chTargetRequest, err1 := target.OpenChannel(userNewChannel.ChannelType(), userNewChannel.ExtraData())
		if err1 != nil {
			log.With("error", err1).Error("ssh open target channel")
			var openErr *ssh.OpenChannelError
			if errors.As(err1, &openErr) {
				userNewChannel.Reject(openErr.Reason, openErr.Message)
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
		defer userChannel.Close()

		var copies sync.WaitGroup
		copies.Add(2)
		// CloseWrite on EOF so the other direction can still deliver data and
		// channel requests such as exit-status.
		go func() {
			defer copies.Done()
			defer userChannel.CloseWrite()
			io.Copy(userChannel, targetChannel)
			log.Info("channel pipe end: from target")
		}()
		go func() {
			defer copies.Done()
			defer targetChannel.CloseWrite()
			io.Copy(targetChannel, userChannel)
			log.Info("channel pipe end: from user")
		}()

		wg1 := &sync.WaitGroup{}
		wg1.Add(2)
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

		copies.Wait()
		// closing the channels unblocks request forwarding if the peer never
		// sends a full close of its own
		_ = userChannel.Close()
		_ = targetChannel.Close()
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
		slots := make(chan struct{}, maxSSHChannels)
		for userNewChannel := range chUserNewChannel {
			select {
			case slots <- struct{}{}:
				wg1.Add(1)
				go func(ch ssh.NewChannel) {
					defer func() { <-slots }()
					handleUserNewChannel(wg1, ch)
				}(userNewChannel)
			default:
				userNewChannel.Reject(ssh.ResourceShortage, "too many channels")
			}
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
