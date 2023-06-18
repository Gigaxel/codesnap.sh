package main

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"time"

	"github.com/gliderlabs/ssh"
	"github.com/google/uuid"
)

var (
	Reset  = "\033[0m"
	Green  = "\033[32m"
	Gray   = "\033[37m"
	Purple = "\033[35m"
)

type SSHServer struct {
	logger Logger
	store  CodeStore
	host   string
}

func NewSSHServer(logger Logger, store CodeStore, host string) *SSHServer {
	return &SSHServer{logger: logger, store: store, host: host}
}

func (s *SSHServer) HandleSession(sess ssh.Session) {
	defer func() {
		err := sess.Close()
		if err != nil {
			s.logger.Errorw("failed to close ssh session", "error", err)
		}
	}()

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

	_, err = sess.Write([]byte(s.genResponse(key)))
	if err != nil {
		s.logger.Errorw("failed to write to ssh session", "error", err)
		return
	}
}

func (s *SSHServer) genResponse(key string) string {
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
	// 100 request per hour
	ssh.Handle(RateLimiter(100, s.HandleSession))
	return ssh.ListenAndServe(addr, handler, options...)
}
