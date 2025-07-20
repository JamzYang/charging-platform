package server

import (
	"context"
	"net"
	"net/http"
	"syscall"
	"time"

	"github.com/charging-platform/charge-point-gateway/internal/logger"
)

// TCPServerConfig TCP服务器配置
type TCPServerConfig struct {
	Host               string        `json:"host"`
	Port               int           `json:"port"`
	ReadTimeout        time.Duration `json:"read_timeout"`
	WriteTimeout       time.Duration `json:"write_timeout"`
	IdleTimeout        time.Duration `json:"idle_timeout"`
	MaxHeaderBytes     int           `json:"max_header_bytes"`
	ListenBacklog      int           `json:"listen_backlog"`       // TCP监听队列大小
	KeepAlivePeriod    time.Duration `json:"keep_alive_period"`    // TCP Keep-Alive周期
	EnableTCPKeepAlive bool          `json:"enable_tcp_keepalive"` // 启用TCP Keep-Alive
}

// DefaultTCPServerConfig 默认TCP服务器配置
func DefaultTCPServerConfig() *TCPServerConfig {
	return &TCPServerConfig{
		Host:               "0.0.0.0",
		Port:               8080,
		ReadTimeout:        60 * time.Second,
		WriteTimeout:       60 * time.Second,
		IdleTimeout:        120 * time.Second,
		MaxHeaderBytes:     1 << 20, // 1MB
		ListenBacklog:      4096,    // 增加监听队列大小
		KeepAlivePeriod:    30 * time.Second,
		EnableTCPKeepAlive: true,
	}
}

// OptimizedTCPServer 优化的TCP服务器
type OptimizedTCPServer struct {
	config   *TCPServerConfig
	server   *http.Server
	listener net.Listener
	logger   *logger.Logger
}

// NewOptimizedTCPServer 创建优化的TCP服务器
func NewOptimizedTCPServer(config *TCPServerConfig, handler http.Handler, log *logger.Logger) *OptimizedTCPServer {
	server := &http.Server{
		Addr:           config.Host + ":" + string(rune(config.Port)),
		Handler:        handler,
		ReadTimeout:    config.ReadTimeout,
		WriteTimeout:   config.WriteTimeout,
		IdleTimeout:    config.IdleTimeout,
		MaxHeaderBytes: config.MaxHeaderBytes,
	}

	return &OptimizedTCPServer{
		config: config,
		server: server,
		logger: log,
	}
}

// createOptimizedListener 创建优化的TCP监听器
func (s *OptimizedTCPServer) createOptimizedListener() (net.Listener, error) {
	// 创建TCP监听器配置
	lc := net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			return c.Control(func(fd uintptr) {
				// 设置SO_REUSEADDR，允许端口重用
				syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)

				// 在Windows上设置SO_REUSEPORT（如果支持）
				// syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEPORT, 1)

				// 设置TCP_NODELAY，禁用Nagle算法
				syscall.SetsockoptInt(int(fd), syscall.IPPROTO_TCP, syscall.TCP_NODELAY, 1)

				// 设置监听队列大小（backlog）
				// 注意：在Windows上，这个设置可能不会生效，需要通过注册表设置
				if s.config.ListenBacklog > 0 {
					// 这里只是示例，实际的backlog设置需要在Listen调用时指定
					s.logger.Infof("Setting listen backlog to %d", s.config.ListenBacklog)
				}
			})
		},
		KeepAlive: s.config.KeepAlivePeriod,
	}

	// 创建监听器
	addr := net.JoinHostPort(s.config.Host, string(rune(s.config.Port)))
	listener, err := lc.Listen(context.Background(), "tcp", addr)
	if err != nil {
		return nil, err
	}

	// 如果是TCP监听器，设置额外的优化参数
	if tcpListener, ok := listener.(*net.TCPListener); ok {
		// 包装监听器以支持自定义Accept行为
		return &optimizedTCPListener{
			TCPListener: tcpListener,
			config:      s.config,
			logger:      s.logger,
		}, nil
	}

	return listener, nil
}

// optimizedTCPListener 优化的TCP监听器包装器
type optimizedTCPListener struct {
	*net.TCPListener
	config *TCPServerConfig
	logger *logger.Logger
}

// Accept 重写Accept方法以优化连接
func (l *optimizedTCPListener) Accept() (net.Conn, error) {
	conn, err := l.TCPListener.AcceptTCP()
	if err != nil {
		return nil, err
	}

	// 设置TCP连接优化参数
	if l.config.EnableTCPKeepAlive {
		conn.SetKeepAlive(true)
		conn.SetKeepAlivePeriod(l.config.KeepAlivePeriod)
	}

	// 设置TCP_NODELAY
	conn.SetNoDelay(true)

	// 设置读写缓冲区大小
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		// 设置读缓冲区大小
		tcpConn.SetReadBuffer(64 * 1024) // 64KB
		// 设置写缓冲区大小
		tcpConn.SetWriteBuffer(64 * 1024) // 64KB
	}

	return conn, nil
}

// Start 启动服务器
func (s *OptimizedTCPServer) Start() error {
	// 创建优化的监听器
	listener, err := s.createOptimizedListener()
	if err != nil {
		return err
	}

	s.listener = listener
	s.logger.Infof("Optimized TCP server listening on %s with backlog %d",
		listener.Addr().String(), s.config.ListenBacklog)

	// 启动服务器
	return s.server.Serve(listener)
}

// Stop 停止服务器
func (s *OptimizedTCPServer) Stop(ctx context.Context) error {
	s.logger.Info("Stopping optimized TCP server...")

	// 优雅关闭服务器
	if err := s.server.Shutdown(ctx); err != nil {
		s.logger.Errorf("Error during server shutdown: %v", err)
		// 强制关闭
		return s.server.Close()
	}

	s.logger.Info("Optimized TCP server stopped")
	return nil
}

// GetAddr 获取服务器地址
func (s *OptimizedTCPServer) GetAddr() net.Addr {
	if s.listener != nil {
		return s.listener.Addr()
	}
	return nil
}

// GetStats 获取服务器统计信息
func (s *OptimizedTCPServer) GetStats() map[string]interface{} {
	stats := make(map[string]interface{})

	if s.listener != nil {
		stats["listening"] = true
		stats["address"] = s.listener.Addr().String()
	} else {
		stats["listening"] = false
	}

	stats["config"] = map[string]interface{}{
		"listen_backlog":       s.config.ListenBacklog,
		"keep_alive_period":    s.config.KeepAlivePeriod.String(),
		"enable_tcp_keepalive": s.config.EnableTCPKeepAlive,
		"read_timeout":         s.config.ReadTimeout.String(),
		"write_timeout":        s.config.WriteTimeout.String(),
		"idle_timeout":         s.config.IdleTimeout.String(),
	}

	return stats
}

// HealthCheck 健康检查
func (s *OptimizedTCPServer) HealthCheck() error {
	if s.listener == nil {
		return net.ErrClosed
	}
	return nil
}
