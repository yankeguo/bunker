package bunker

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"

	"github.com/yankeguo/bunker/model"
	"github.com/yankeguo/bunker/model/dao"
	"golang.org/x/crypto/ssh"
	"gorm.io/gorm/clause"
)

const (
	hostKeyAccept = "accept"
	hostKeyRecord = "record"
	hostKeyReject = "reject"
)

type knownHostKey struct {
	KeyType string
	Raw     []byte
}

func hostKeyID(serverID, keyType string) string {
	sum := sha256.Sum256([]byte(serverID + "\n" + keyType))
	return hex.EncodeToString(sum[:])
}

// classifyHostKey compares a presented host key with keys already pinned for
// that server. A new algorithm is recorded; a changed key of a known
// algorithm is rejected.
func classifyHostKey(known []knownHostKey, key ssh.PublicKey) string {
	raw := key.Marshal()
	keyType := key.Type()
	for _, item := range known {
		if item.KeyType != keyType {
			continue
		}
		if bytes.Equal(item.Raw, raw) {
			return hostKeyAccept
		}
		return hostKeyReject
	}
	return hostKeyRecord
}

func (s *SSHServer) hostKeyCallback(serverID string) ssh.HostKeyCallback {
	return func(_ string, _ net.Addr, key ssh.PublicKey) error {
		return s.verifyHostKey(serverID, key)
	}
}

func (s *SSHServer) verifyHostKey(serverID string, key ssh.PublicKey) error {
	db := dao.Use(s.db)

	rows, err := db.HostKey.Where(db.HostKey.ServerID.Eq(serverID)).Find()
	if err != nil {
		return err
	}

	known := make([]knownHostKey, 0, len(rows))
	for _, row := range rows {
		parsed, _, _, _, parseErr := ssh.ParseAuthorizedKey([]byte(row.PublicKey))
		if parseErr != nil {
			s.log.With("server_id", serverID, "error", parseErr).Error("stored host key is unreadable")
			continue
		}
		known = append(known, knownHostKey{KeyType: parsed.Type(), Raw: parsed.Marshal()})
	}

	fingerprint := ssh.FingerprintSHA256(key)
	switch classifyHostKey(known, key) {
	case hostKeyAccept:
		return nil
	case hostKeyReject:
		s.log.With("server_id", serverID, "key_type", key.Type(), "fingerprint", fingerprint).Warn("ssh host key changed")
		return fmt.Errorf("host key for server %q changed (%s); reset the recorded host key if this server was reinstalled", serverID, fingerprint)
	}

	row := &model.HostKey{
		ID:          hostKeyID(serverID, key.Type()),
		ServerID:    serverID,
		KeyType:     key.Type(),
		Fingerprint: fingerprint,
		PublicKey:   string(ssh.MarshalAuthorizedKey(key)),
	}
	if err = db.HostKey.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoNothing: true,
	}).Create(row); err != nil {
		return err
	}

	stored, err := db.HostKey.Where(db.HostKey.ID.Eq(row.ID)).First()
	if err != nil {
		return err
	}
	parsed, _, _, _, err := ssh.ParseAuthorizedKey([]byte(stored.PublicKey))
	if err != nil {
		return fmt.Errorf("stored host key for server %q is unreadable", serverID)
	}
	if !bytes.Equal(parsed.Marshal(), key.Marshal()) {
		s.log.With("server_id", serverID, "key_type", key.Type(), "fingerprint", fingerprint).Warn("ssh host key changed")
		return fmt.Errorf("host key for server %q changed (%s); reset the recorded host key if this server was reinstalled", serverID, fingerprint)
	}

	s.log.With("server_id", serverID, "key_type", key.Type(), "fingerprint", fingerprint).Info("ssh host key recorded")
	return nil
}
