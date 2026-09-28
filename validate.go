package bunker

import (
	"crypto/rsa"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/ssh"
)

const (
	maxServerIDLen    = 128
	maxGrantPattern   = 128
	maxDisplayNameLen = 64
	maxPublicKeyLen   = 16 << 10
	maxUserAgentLen   = 256
)

func validateServerID(id string) error {
	if id == "" {
		return errors.New("server id is required")
	}
	if len(id) > maxServerIDLen {
		return errors.New("server id is too long")
	}
	if strings.ContainsAny(id, " \t\r\n@*?[]/") {
		return errors.New("server id must not contain whitespace, @, /, or wildcard characters")
	}
	return nil
}

func validateGrantPattern(fieldName, value string) error {
	if value == "" {
		return fmt.Errorf("%s is required", fieldName)
	}
	if len(value) > maxGrantPattern {
		return fmt.Errorf("%s is too long", fieldName)
	}
	if strings.ContainsAny(value, " \t\r\n@/") {
		return fmt.Errorf("%s must not contain whitespace, @, or /", fieldName)
	}
	return nil
}

func normalizeDisplayName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "Unnamed", nil
	}
	if utf8.RuneCountInString(name) > maxDisplayNameLen {
		return "", errors.New("display name is too long")
	}
	if strings.ContainsAny(name, "\r\n") {
		return "", errors.New("display name must not contain line breaks")
	}
	return name, nil
}

func normalizeUserAgent(ua string) string {
	ua = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(ua, "\n", " "), "\r", " "))
	if len(ua) > maxUserAgentLen {
		ua = ua[:maxUserAgentLen]
	}
	return ua
}

// normalizeServerAddress accepts a host, host:port, IPv6, or [IPv6]:port and
// returns a dialable host:port. A missing port defaults to 22.
func normalizeServerAddress(address string) (string, error) {
	address = strings.TrimSpace(address)
	if address == "" || strings.ContainsAny(address, " \t\r\n") {
		return "", errors.New("invalid server address")
	}

	host, port, err := splitHostOptionalPort(address)
	if err != nil {
		return "", err
	}

	portNum, err := strconv.Atoi(port)
	if err != nil || portNum < 1 || portNum > 65535 {
		return "", errors.New("invalid server port")
	}
	port = strconv.Itoa(portNum)

	if ip := net.ParseIP(host); ip != nil {
		return net.JoinHostPort(ip.String(), port), nil
	}

	if len(host) > 253 || strings.Contains(host, "..") {
		return "", errors.New("invalid server address")
	}
	for _, label := range strings.Split(host, ".") {
		if label == "" || len(label) > 63 {
			return "", errors.New("invalid server address")
		}
		for _, r := range label {
			if r == '-' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
				continue
			}
			return "", errors.New("invalid server address")
		}
	}

	return net.JoinHostPort(host, port), nil
}

func splitHostOptionalPort(address string) (host, port string, err error) {
	if h, p, splitErr := net.SplitHostPort(address); splitErr == nil {
		if h == "" {
			return "", "", errors.New("invalid server address")
		}
		return h, p, nil
	}

	if strings.HasPrefix(address, "[") && strings.HasSuffix(address, "]") {
		host = strings.TrimSuffix(strings.TrimPrefix(address, "["), "]")
		if host == "" {
			return "", "", errors.New("invalid server address")
		}
		return host, "22", nil
	}

	// A bare IPv6 address has more than one colon and no port.
	if strings.Count(address, ":") > 1 {
		return address, "22", nil
	}
	if strings.Contains(address, ":") {
		return "", "", errors.New("invalid server address")
	}
	return address, "22", nil
}

func parseUserPublicKey(raw string) (ssh.PublicKey, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errors.New("public key is required")
	}
	if len(raw) > maxPublicKeyLen {
		return nil, errors.New("public key is too large")
	}

	var lines []string
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		lines = append(lines, line)
	}
	if len(lines) != 1 {
		return nil, errors.New("paste a single public key")
	}

	key, _, _, _, err := ssh.ParseAuthorizedKey([]byte(lines[0]))
	if err != nil {
		return nil, fmt.Errorf("invalid public key: %w", err)
	}
	if err = validatePublicKey(key); err != nil {
		return nil, err
	}
	return key, nil
}

func validatePublicKey(key ssh.PublicKey) error {
	switch key.Type() {
	case ssh.KeyAlgoED25519, ssh.KeyAlgoSKED25519, ssh.KeyAlgoECDSA256, ssh.KeyAlgoSKECDSA256, ssh.KeyAlgoECDSA384, ssh.KeyAlgoECDSA521:
		return nil
	case ssh.KeyAlgoRSA:
		cryptoKey, ok := key.(ssh.CryptoPublicKey)
		if !ok {
			return errors.New("unsupported public key")
		}
		rsaKey, ok := cryptoKey.CryptoPublicKey().(*rsa.PublicKey)
		if !ok || rsaKey.N == nil || rsaKey.N.BitLen() < 2048 {
			return errors.New("RSA public keys must be at least 2048 bits")
		}
		return nil
	default:
		return fmt.Errorf("unsupported public key type %s", key.Type())
	}
}

// parseSSHTarget splits the SSH username "server_user@server_id".
func parseSSHTarget(user string) (serverUser, serverID string, err error) {
	serverUser, serverID, ok := strings.Cut(user, "@")
	if !ok || serverUser == "" || serverID == "" || strings.Contains(serverID, "@") {
		err = errors.New("invalid user format, should be server_user@server_id")
		return
	}
	if strings.ContainsAny(serverUser, " \t\r\n") || strings.ContainsAny(serverID, " \t\r\n") {
		err = errors.New("invalid user format, should be server_user@server_id")
		return
	}
	return
}
