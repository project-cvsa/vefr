package proxy

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"vefr/internal/config"
	"vefr/internal/ippool"
)

type Server struct {
	cfg       config.Config
	pool      *ippool.Pool
	logger    *slog.Logger
	requests  atomic.Uint64
	active    atomic.Int64
	transport *http.Transport
}

func NewServer(cfg config.Config, pool *ippool.Pool, logger *slog.Logger) *Server {
	s := &Server{cfg: cfg, pool: pool, logger: logger}
	s.transport = &http.Transport{
		Proxy:             nil,
		ForceAttemptHTTP2: false,
		// A fresh upstream connection makes source-IP rotation effective per
		// HTTP request. CONNECT tunnels are always dedicated connections.
		DisableKeepAlives:     true,
		DialContext:           s.dialContext,
		TLSHandshakeTimeout:   cfg.Timeouts.Connect,
		ResponseHeaderTimeout: cfg.Timeouts.Request,
		IdleConnTimeout:       cfg.Timeouts.Idle,
	}
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.requests.Add(1)
	s.active.Add(1)
	defer s.active.Add(-1)
	if r.URL.Path == "/healthz" && r.Method == http.MethodGet {
		s.handleHealth(w)
		return
	}
	if !s.authorized(r) {
		w.Header().Set("Proxy-Authenticate", `Basic realm="vefr"`)
		http.Error(w, "proxy authentication required", http.StatusProxyAuthRequired)
		return
	}
	if r.Method == http.MethodConnect {
		s.handleConnect(w, r)
		return
	}
	s.handleHTTP(w, r)
}

func (s *Server) authorized(r *http.Request) bool {
	if s.cfg.AuthEnabled != nil && !*s.cfg.AuthEnabled {
		return true
	}
	if s.cfg.Username == "" && s.cfg.Password == "" {
		return false
	}
	user, pass, ok := r.BasicAuth()
	if !ok {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(user), []byte(s.cfg.Username)) == 1 && subtle.ConstantTimeCompare([]byte(pass), []byte(s.cfg.Password)) == 1
}

func (s *Server) handleHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.IsAbs() == false {
		http.Error(w, "proxy requests require an absolute URL", http.StatusBadRequest)
		return
	}
	target, err := normalizeTarget(r.URL.Host, "80")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	r.URL.Host = target
	if err := s.validateTarget(target); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	r.RequestURI = ""
	r.Header.Del("Proxy-Authorization")
	r.Header.Del("Proxy-Connection")
	resp, err := s.transport.RoundTrip(r)
	if err != nil {
		s.logger.Warn("proxy request failed", "target", r.URL.Host, "error", err)
		http.Error(w, "upstream request failed", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

func (s *Server) handleConnect(w http.ResponseWriter, r *http.Request) {
	if err := s.validateTarget(r.Host); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	dialCtx, cancel := context.WithTimeout(r.Context(), s.cfg.Timeouts.Connect)
	defer cancel()
	upstream, err := s.dialContext(dialCtx, "tcp", r.Host)
	if err != nil {
		s.logger.Warn("connect failed", "target", r.Host, "error", err)
		http.Error(w, "upstream connection failed", http.StatusBadGateway)
		return
	}
	defer upstream.Close()
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "hijacking is unsupported", http.StatusInternalServerError)
		return
	}
	client, rw, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, "hijacking failed", http.StatusInternalServerError)
		return
	}
	defer client.Close()
	if _, err := rw.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
		return
	}
	if err := rw.Flush(); err != nil {
		return
	}
	if rw.Reader.Buffered() > 0 {
		_, _ = io.CopyN(upstream, rw, int64(rw.Reader.Buffered()))
	}
	join := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(upstream, client); _ = closeWrite(upstream); join <- struct{}{} }()
	go func() { _, _ = io.Copy(client, upstream); _ = closeWrite(client); join <- struct{}{} }()
	<-join
}

func (s *Server) dialContext(ctx context.Context, network, address string) (net.Conn, error) {
	source := s.pool.Next()
	dialer := &net.Dialer{Timeout: s.cfg.Timeouts.Connect, KeepAlive: 30 * time.Second, LocalAddr: &net.TCPAddr{IP: source}}
	return dialer.DialContext(ctx, network, address)
}

func (s *Server) validateTarget(hostport string) error {
	host, portText, err := net.SplitHostPort(hostport)
	if err != nil {
		return fmt.Errorf("invalid target: %s", hostport)
	}
	port, err := strconv.Atoi(portText)
	if err != nil || !s.allowedPort(port) {
		return fmt.Errorf("target port is not allowed")
	}
	host = strings.Trim(host, "[]")
	if s.cfg.BlockPrivate != nil && !*s.cfg.BlockPrivate {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
	if err != nil || len(ips) == 0 {
		return fmt.Errorf("target DNS lookup failed")
	}
	for _, ip := range ips {
		if isPrivateOrLocal(ip) {
			return fmt.Errorf("private or local targets are blocked")
		}
	}
	return nil
}

func normalizeTarget(raw, defaultPort string) (string, error) {
	if _, _, err := net.SplitHostPort(raw); err == nil {
		return raw, nil
	}
	host := strings.Trim(raw, "[]")
	if host == "" || (strings.Contains(host, ":") && net.ParseIP(host) == nil) {
		return "", fmt.Errorf("invalid target: %s", raw)
	}
	return net.JoinHostPort(host, defaultPort), nil
}

func (s *Server) allowedPort(port int) bool {
	for _, allowed := range s.cfg.AllowPorts {
		if port == allowed {
			return true
		}
	}
	return false
}

func isPrivateOrLocal(ip net.IP) bool {
	if ip == nil || ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast() {
		return true
	}
	if ip4 := ip.To4(); ip4 != nil && ip4[0] == 169 && ip4[1] == 254 {
		return true
	}
	return false
}

func closeWrite(conn net.Conn) error {
	if tcp, ok := conn.(*net.TCPConn); ok {
		return tcp.CloseWrite()
	}
	return nil
}

func (s *Server) handleHealth(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "requests": s.requests.Load(), "active": s.active.Load(), "sources": s.pool.Size()})
}
