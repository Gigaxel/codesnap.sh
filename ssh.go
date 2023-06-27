package main

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"github.com/gliderlabs/ssh"
	"github.com/google/uuid"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

var (
	Reset  = "\033[0m"
	Red    = "\033[31m"
	Green  = "\033[32m"
	Gray   = "\033[37m"
	Purple = "\033[35m"
)

const (
	MaxUploadSize        = 1024 * 1024     // 1 MB
	MaxStreamSize        = 5 * 1024 * 1024 // 5 MB
	MaxTTL               = 24 * time.Hour
	MinTTL               = 60 * time.Second
	KeyLength            = 7
	CodeUploadedCountKey = "code_uploaded_count"
)

type SSHServer struct {
	logger        Logger
	store         Store
	tunnelManager *TunnelManager
	rateLimiter   *RateLimiter
	host          string
}

func NewSSHServer(
	logger Logger,
	store Store,
	rateLimiter *RateLimiter,
	tunnelManager *TunnelManager,
	host string,
) *SSHServer {
	return &SSHServer{
		logger:        logger,
		store:         store,
		rateLimiter:   rateLimiter,
		tunnelManager: tunnelManager,
		host:          host,
	}
}

func (s *SSHServer) parseCommand(cmd string) (Command, string) {
	parts := strings.Split(cmd, "=")
	if len(parts) != 2 {
		return CmdUnknown, ""
	}
	return ParseCmd(parts[0]), parts[1]
}

func (s *SSHServer) handleTunnelCommand(sess ssh.Session) {
	key := s.genKey()
	s.logger.Debugw("creating tunnel", "key", key)

	tunnel := NewTunnelData(sess)
	s.tunnelManager.AddTunnel(key, tunnel)

	_, err := sess.Write([]byte(s.genTunnelCreatedResponse(key)))
	if err != nil {
		s.logger.Errorw("failed to write to ssh session", "error", err)
		return
	}

	tunnel.Wait()
	s.tunnelManager.RemoveTunnel(key)

	s.logger.Debugw("transfer over tunnel complete", "key", key)

	_, err = sess.Write([]byte(s.genDataTransferredOverTunnelResponse()))
	if err != nil {
		s.logger.Errorw("failed to write to ssh session", "error", err)
		return
	}
}

func (s *SSHServer) handleSessionWithCommand(sess ssh.Session) {
	cmd := sess.Command()[0]
	s.logger.Debugw("received command", "command", cmd)

	cmdType, cmdValue := s.parseCommand(cmd)
	switch cmdType {
	case CmdTunnel:
		shouldTunnel, err := strconv.ParseBool(cmdValue)
		if err != nil {
			s.logger.Errorw("failed to parse tunnel command", "error", err)
			return
		}
		if !shouldTunnel {
			s.handleBasicSession(sess, MaxTTL)
			return
		}
		s.handleTunnelCommand(sess)
	case CmdTTL:
		userTTL, err := strconv.Atoi(cmdValue)
		if err != nil {
			s.logger.Errorw("failed to parse ttl command", "error", err)
			return
		}
		s.handleTTLCommand(sess, userTTL)
	default:
		s.logger.Warnw("unknown command", "command", cmd)
	}
}

func (s *SSHServer) handleTTLCommand(sess ssh.Session, userTTL int) {
	ttl := time.Duration(userTTL) * time.Second
	if ttl < MinTTL {
		s.logger.Debugw("user ttl too low; falling back on MinTTL", "ttl", ttl, "min_ttl", MinTTL)
		ttl = MinTTL
	}
	if ttl > MaxTTL {
		s.logger.Debugw("user ttl too high; falling back on MaxTTL", "ttl", ttl, "max_ttl", MaxTTL)
		ttl = MaxTTL
	}
	s.handleBasicSession(sess, ttl)
}

func (s *SSHServer) handleBasicSession(sess ssh.Session, ttl time.Duration) {
	buf := make([]byte, MaxUploadSize)
	// read from ssh session 1MB at a time
	n, err := io.ReadFull(sess, buf)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		s.logger.Errorw("failed to read from ssh session", "error", err)
		return
	}

	key := s.genKey()
	s.logger.Debugw("writing to store", "key", key, "code", string(buf[:n]))
	err = s.store.Set(sess.Context(), key, string(buf[:n]), ttl)
	s.logger.Debugw("wrote bytes", "bytes", n)
	if err != nil {
		s.logger.Errorw("failed to write to redis",
			"error",
			err,
			"key",
			key,
			"code",
			string(buf[:n]),
		)
		return
	}
	_, err = s.store.Incr(sess.Context(), CodeUploadedCountKey)
	if err != nil {
		s.logger.Errorw("failed to increment code uploaded count", "error", err)
		return
	}

	_, err = sess.Write([]byte(s.genBasicResponse(key)))
	if err != nil {
		s.logger.Errorw("failed to write to ssh session", "error", err)
		return
	}

}

func (s *SSHServer) isRateLimited(sess ssh.Session) (bool, error) {
	ip, _, err := net.SplitHostPort(sess.RemoteAddr().String())
	if err != nil {
		return false, err
	}

	limited, err := s.rateLimiter.IsRateLimited(sess.Context(), ip)
	if err != nil {
		return false, err
	}

	return limited, nil
}

func (s *SSHServer) HandleSession(sess ssh.Session) {
	defer func() {
		err := sess.Close()
		if err != nil {
			s.logger.Errorw("failed to close ssh session", "error", err)
		}
	}()

	rateLimited, err := s.isRateLimited(sess)
	if err != nil {
		s.logger.Errorw("failed to check rate limit", "error", err)
		return
	}
	if rateLimited {
		s.logger.Infow("rate limited", "ip", sess.RemoteAddr().String())
		_, err = sess.Write([]byte(s.genRateLimitedResponse()))
		if err != nil {
			s.logger.Errorw("failed to write to ssh session", "error", err)
			return
		}
		return
	}

	if len(sess.Command()) > 0 {
		s.handleSessionWithCommand(sess)
		return
	}

	s.handleBasicSession(sess, MaxTTL)
}

func (s *SSHServer) genDataTransferredOverTunnelResponse() string {
	output := fmt.Sprintf("%sCode transferred successfully! %s\n\n", Green, Reset)
	return output
}

func (s *SSHServer) genTunnelCreatedResponse(key string) string {
	output := fmt.Sprintf("%s+------------------------+\n", Gray)
	output += fmt.Sprintf("|    💻 codesnap.sh 💻   |\n")
	output += fmt.Sprintf("+------------------------+%s\n\n", Reset)

	output += fmt.Sprintf("%sYou opened a tunnel. Your code is ready to be streamed! 🚀%s\n\n", Green, Reset)

	linkToCode := fmt.Sprintf("%s/t/%s", s.host, key)
	link := fmt.Sprintf("%s%s%s", Purple, linkToCode, Reset)
	output += fmt.Sprintf("Link: %s\n\n", link)

	return output
}

func (s *SSHServer) genBasicResponse(key string) string {
	output := fmt.Sprintf("%s+------------------------+\n", Gray)
	output += fmt.Sprintf("|    💻 codesnap.sh 💻   |\n")
	output += fmt.Sprintf("+------------------------+%s\n\n", Reset)

	output += fmt.Sprintf("%sYour code has been successfully uploaded! 🚀%s\n\n", Green, Reset)

	linkToCode := fmt.Sprintf("%s/c/%s", s.host, key)
	link := fmt.Sprintf("%s%s%s", Purple, linkToCode, Reset)
	output += fmt.Sprintf("Link: %s\n\n", link)

	output += fmt.Sprintf("%s+------------------------+\n", Green)

	return output
}

func (s *SSHServer) genRateLimitedResponse() string {
	output := fmt.Sprintf("%s+------------------------+\n", Gray)
	output += fmt.Sprintf("|    💻 codesnap.sh 💻   |\n")
	output += fmt.Sprintf("+------------------------+%s\n\n", Reset)

	output += fmt.Sprintf("%sYou have been rate limited. Please try again later.%s\n\n", Red, Reset)

	output += fmt.Sprintf("%s+------------------------+\n", Green)

	return output
}

func (s *SSHServer) genKey() string {
	id := uuid.New()
	h := sha1.New()
	h.Write([]byte(id.String()))
	return hex.EncodeToString(h.Sum(nil))[:KeyLength]
}

func (s *SSHServer) ListenAndServe(addr string, handler ssh.Handler, options ...ssh.Option) error {
	ssh.Handle(s.HandleSession)
	return ssh.ListenAndServe(addr, handler, options...)
}
