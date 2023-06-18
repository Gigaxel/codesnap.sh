package main

import (
	"context"
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
	Green  = "\033[32m"
	Gray   = "\033[37m"
	Purple = "\033[35m"
)

const (
	MaxUploadSize = 1024 * 1024     // 1 MB
	MaxStreamSize = 5 * 1024 * 1024 // 5 MB
	// specify how many request a client can send in an hour
	rateLimiter = 100
)

type SSHServer struct {
	logger        Logger
	store         CodeStore
	tunnelManager *TunnelManager
	host          string
}

// rate limiter
func (s *SSHServer) requestCounter(sess ssh.Session) bool {
	clientIP := sess.RemoteAddr().(*net.TCPAddr).IP.String()
	// if client exists in redis -> exists = 1, else -> exists = 0
	exists, err := s.store.Exists(sess.Context(), clientIP)
	if err != nil {
		s.logger.Errorw("Error checking client exists in redis:", "error", err)
		return false
	}
	// create client in redis for the first time
	if exists == 0 {
		// set the expiration time to one hour, so after an hour, client's information will be deleted from redis
		// note that every one hour the client will be removed from redis and if a new request is sent, it will be created again
		err := s.store.Set(sess.Context(), clientIP, 1, time.Hour)
		if err != nil {
			s.logger.Errorw("Error setting the client in redis:", "error", err)
			return false
		}
	} else {
		// get the value of clientIP in redis
		value, err := s.store.Get(context.Background(), clientIP)

		if err != nil {
			s.logger.Errorw("Error getting the number of requests of the client:", "error", err)
			return false
		}
		// convert value of Get to integer
		counter, err := strconv.Atoi(string(value))
		if err != nil {
			s.logger.Errorw("Error getting a string from redis instead of a number:", "error", err)
			return false
		}
		// client has sent more than 100 requests for the last hour
		if counter > rateLimiter {
			// block the client for one hour
			err = s.store.Set(sess.Context(), clientIP, -1, time.Hour)
			if err != nil {
				s.logger.Errorw("Error blocking the client in redis:", "error", err)
				return false
			}
			s.logger.Warnw("Blocked client due to excessive SSH requests", "clientIP", clientIP)
			// terminate
			return false
		} else if counter > 0 {
			// counter between 0 and 100, so proceed to provide service ...
			// Increment the request count for the client in Redis
			_, err = s.store.Incr(sess.Context(), clientIP)
			if err != nil {
				s.logger.Errorw("Error incrementing request count:", "error", err)
				return false
			}
		} else {
			// counter <= 0 meaning that counter is equal to -1 and the client is still blocked
			s.logger.Warnw("Blocked client attempted SSH connection", "clientIP", clientIP)
			return false
		}
	}
	return true
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
	buf := make([]byte, MaxUploadSize)
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

	// call rate limiter and do not proceed if the client was blocked
	if !s.requestCounter(sess) {
		return
	}

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
