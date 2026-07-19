package proxy

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/elazarl/goproxy"
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
	proxy     *goproxy.ProxyHttpServer
	sourceMu  sync.Mutex
	sources   map[string]sourceStatus
}

type sourceStatus struct {
	failures       int
	unhealthyUntil time.Time
}

type activityConn struct {
	net.Conn
	idle time.Duration
}

func (c *activityConn) Read(p []byte) (int, error) {
	c.refreshReadDeadline()
	return c.Conn.Read(p)
}

func (c *activityConn) Write(p []byte) (int, error) {
	c.refreshWriteDeadline()
	return c.Conn.Write(p)
}

func (c *activityConn) CloseRead() error {
	if conn, ok := c.Conn.(interface{ CloseRead() error }); ok {
		return conn.CloseRead()
	}
	return nil
}

func (c *activityConn) CloseWrite() error {
	if conn, ok := c.Conn.(interface{ CloseWrite() error }); ok {
		return conn.CloseWrite()
	}
	return nil
}

func (c *activityConn) refreshReadDeadline() {
	if c.idle > 0 {
		deadline := time.Now().Add(c.idle)
		_ = c.Conn.SetReadDeadline(deadline)
		_ = c.Conn.SetWriteDeadline(deadline)
	}
}

func (c *activityConn) refreshWriteDeadline() {
	if c.idle > 0 {
		deadline := time.Now().Add(c.idle)
		_ = c.Conn.SetReadDeadline(deadline)
		_ = c.Conn.SetWriteDeadline(deadline)
	}
}

func NewServer(cfg config.Config, pool *ippool.Pool, logger *slog.Logger) *Server {
	s := &Server{cfg: cfg, pool: pool, logger: logger, sources: make(map[string]sourceStatus)}
	s.transport = &http.Transport{
		Proxy:                 nil,
		ForceAttemptHTTP2:     false,
		MaxIdleConns:          256,
		MaxIdleConnsPerHost:   16,
		MaxConnsPerHost:       64,
		DialContext:           s.dialContext,
		TLSHandshakeTimeout:   cfg.Timeouts.Connect,
		ResponseHeaderTimeout: cfg.Timeouts.Request,
		IdleConnTimeout:       cfg.Timeouts.Idle,
	}
	s.proxy = goproxy.NewProxyHttpServer()
	s.proxy.Tr = s.transport
	s.proxy.ConnectDialWithReq = func(req *http.Request, network, address string) (net.Conn, error) {
		return s.dialContext(req.Context(), network, address)
	}
	s.proxy.ConnectionErrHandler = func(conn io.Writer, ctx *goproxy.ProxyCtx, err error) {
		s.logger.Warn("upstream connection failed", "target", ctx.Req.Host, "error", err)
		_, _ = io.WriteString(conn, "HTTP/1.1 502 Bad Gateway\r\nContent-Type: text/plain\r\nConnection: close\r\n\r\nupstream connection failed\r\n")
	}
	s.proxy.OnResponse().DoFunc(func(resp *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
		if resp != nil {
			return resp
		}
		s.logger.Warn("upstream request failed", "target", ctx.Req.URL.Host, "error", ctx.Error)
		return &http.Response{
			StatusCode: http.StatusBadGateway,
			Status:     "502 Bad Gateway",
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader("upstream request failed\n")),
			Request:    ctx.Req,
		}
	})
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
	if err := s.validateRequestTarget(r); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	s.proxy.ServeHTTP(w, r)
}

func (s *Server) authorized(r *http.Request) bool {
	if s.cfg.AuthEnabled != nil && !*s.cfg.AuthEnabled {
		return true
	}
	if s.cfg.Username == "" && s.cfg.Password == "" {
		return false
	}
	user, pass, ok := proxyBasicAuth(r)
	if !ok {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(user), []byte(s.cfg.Username)) == 1 && subtle.ConstantTimeCompare([]byte(pass), []byte(s.cfg.Password)) == 1
}

func proxyBasicAuth(r *http.Request) (username, password string, ok bool) {
	value := r.Header.Get("Proxy-Authorization")
	scheme, encoded, found := strings.Cut(value, " ")
	if !found || !strings.EqualFold(scheme, "Basic") {
		return "", "", false
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		return "", "", false
	}
	username, password, found = strings.Cut(string(decoded), ":")
	return username, password, found
}

func (s *Server) validateRequestTarget(r *http.Request) error {
	if r.Method == http.MethodConnect {
		target, err := normalizeTarget(r.Host, "443")
		if err != nil {
			return err
		}
		r.Host = target
		r.URL.Host = target
		return s.validateTarget(target)
	}
	if r.URL.IsAbs() == false {
		return fmt.Errorf("proxy requests require an absolute URL")
	}
	target, err := normalizeTarget(r.URL.Host, "80")
	if err != nil {
		return err
	}
	r.URL.Host = target
	return s.validateTarget(target)
}

func (s *Server) dialContext(ctx context.Context, network, address string) (net.Conn, error) {
	var lastErr error
	excluded := make(map[string]struct{})
	for attempt := 0; attempt < 3; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		source := s.nextSource(excluded)
		excluded[source.String()] = struct{}{}
		dialer := &net.Dialer{Timeout: s.cfg.Timeouts.Connect, KeepAlive: 30 * time.Second, LocalAddr: &net.TCPAddr{IP: source}}
		conn, err := dialer.DialContext(ctx, network, address)
		if err == nil {
			s.markSourceSuccess(source)
			return &activityConn{Conn: conn, idle: s.cfg.Timeouts.Idle}, nil
		}
		s.markSourceFailure(source)
		lastErr = fmt.Errorf("source %s: %w", source, err)
		s.logger.Warn("source dial failed", "source", source, "target", address, "attempt", attempt+1, "error", err)
	}
	return nil, lastErr
}

func (s *Server) nextSource(excluded map[string]struct{}) net.IP {
	var fallback net.IP
	for i := 0; i < 8; i++ {
		source := s.pool.Next()
		if fallback == nil {
			fallback = source
		}
		key := source.String()
		if _, ok := excluded[key]; ok {
			continue
		}
		s.sourceMu.Lock()
		status := s.sources[key]
		healthy := status.unhealthyUntil.Before(time.Now())
		s.sourceMu.Unlock()
		if healthy {
			return source
		}
	}
	return fallback
}

func (s *Server) markSourceSuccess(source net.IP) {
	s.sourceMu.Lock()
	delete(s.sources, source.String())
	s.sourceMu.Unlock()
}

func (s *Server) markSourceFailure(source net.IP) {
	key := source.String()
	s.sourceMu.Lock()
	status := s.sources[key]
	status.failures++
	if status.failures >= 2 {
		status.unhealthyUntil = time.Now().Add(30 * time.Second)
	}
	s.sources[key] = status
	s.sourceMu.Unlock()
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

func (s *Server) handleHealth(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "requests": s.requests.Load(), "active": s.active.Load(), "sources": s.pool.Size()})
}
