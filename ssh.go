package main

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"github.com/gliderlabs/ssh"
	"github.com/google/uuid"
	"io"
	"strconv"
	"strings"
	"time"
)

var (
	Reset  = "\033[0m"
	Green  = "\033[32m"
	Blue   = "\033[34m"
	Red    = "\033[31m"
	Gray   = "\033[37m"
	Purple = "\033[35m"
)

type SSHServer struct {
	logger        Logger
	store         CodeStore
	tunnelManager *TunnelManager
	host          string
}

func NewSSHServer(logger Logger, store CodeStore, tunnelManager *TunnelManager, host string) *SSHServer {
	return &SSHServer{logger: logger, store: store, tunnelManager: tunnelManager, host: host}
}

func (s *SSHServer) parseCommand(cmd string) (string, string) {
	parts := strings.Split(cmd, "=")
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
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

	cmdKey, cmdValue := s.parseCommand(cmd)
	switch cmdKey {
	case "tunnel":
		shouldTunnel, err := strconv.ParseBool(cmdValue)
		if err != nil {
			s.logger.Errorw("failed to parse tunnel command", "error", err)
			return
		}
		if !shouldTunnel {
			s.handleBasicSession(sess)
			return
		}
		s.handleTunnelCommand(sess)
	default:
		s.logger.Warnw("unknown command", "command", cmd)
	}
}

func (s *SSHServer) handleBasicSession(sess ssh.Session) {
	buf := make([]byte, 1024*1000) // 1MB max size
	// read from ssh session 1MB at a time
	n, err := io.ReadFull(sess, buf)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		s.logger.Errorw("failed to read from ssh session", "error", err)
		return
	}

	key := s.genKey()
	s.logger.Debugw("writing to store", "key", key, "code", string(buf[:n]))
	err = s.store.Set(sess.Context(), key, string(buf[:n]), time.Hour*24)
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

	_, err = sess.Write([]byte(s.genBasicResponse(key)))
	if err != nil {
		s.logger.Errorw("failed to write to ssh session", "error", err)
		return
	}
}

func (s *SSHServer) HandleSession(sess ssh.Session) {
	defer func() {
		err := sess.Close()
		if err != nil {
			s.logger.Errorw("failed to close ssh session", "error", err)
		}
	}()

	if len(sess.Command()) > 0 {
		s.handleSessionWithCommand(sess)
		return
	}

	s.handleBasicSession(sess)
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

func (s *SSHServer) genKey() string {
	id := uuid.New()
	h := sha1.New()
	h.Write([]byte(id.String()))
	return hex.EncodeToString(h.Sum(nil))[:7]
}

func (s *SSHServer) ListenAndServe(addr string, handler ssh.Handler, options ...ssh.Option) error {
	ssh.Handle(s.HandleSession)
	return ssh.ListenAndServe(addr, handler, options...)
}
