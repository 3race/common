package rtmp

import (
	"fmt"
	"net"
	"time"

	"github.com/studease/common/log"
	rtmpcfg "github.com/studease/common/rtmp/config"
	"github.com/studease/common/utils"
)

// Static constants
const (
	DEFAULT_PORT             = 1935
	DEFAULT_TIMEOUT          = 10
	DEFAULT_SEND_BUFFER_SIZE = 65536
	DEFAULT_READ_BUFFER_SIZE = 65536
	DEFAULT_ROOT             = "applications"
	DEFAULT_CORS             = "webroot/crossdomain.xml"
	DEFAULT_TARGET           = "conf/target.xml"
	DEFAULT_CHUNK_SIZE       = 4096
	DEFAULT_MAX_IDLE_TIME    = 3600
	DEFAULT_ACK_WINDOW_SIZE  = 2500000
	DEFAULT_PEER_BANDWIDTH   = 2500000
)

// Server defines parameters for running an RTMP server
type Server struct {
	CFG     *rtmpcfg.Server
	Mux     utils.Mux
	logger  log.ILogger
	factory log.ILoggerFactory
}

// Init this class
func (me *Server) Init(cfg *rtmpcfg.Server, logger log.ILogger, factory log.ILoggerFactory) *Server {
	me.Mux.Init()
	me.CFG = cfg
	me.logger = logger
	me.factory = factory

	if cfg.Port == 0 {
		cfg.Port = DEFAULT_PORT
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = DEFAULT_TIMEOUT
	}
	if cfg.MaxIdleTime == 0 {
		cfg.MaxIdleTime = DEFAULT_MAX_IDLE_TIME
	}
	if cfg.SendBufferSize == 0 {
		cfg.SendBufferSize = DEFAULT_SEND_BUFFER_SIZE
	}
	if cfg.ReadBufferSize == 0 {
		cfg.ReadBufferSize = DEFAULT_READ_BUFFER_SIZE
	}
	if cfg.Root == "" {
		cfg.Root = DEFAULT_ROOT
	}
	if cfg.Cors == "" {
		cfg.Cors = DEFAULT_CORS
	}
	if cfg.Target == "" {
		cfg.Target = DEFAULT_TARGET
	}
	if cfg.ChunkSize < 128 || cfg.ChunkSize > 65536 {
		cfg.ChunkSize = DEFAULT_CHUNK_SIZE
	}

	return me
}

// ListenAndServe listens on the TCP network address and then calls Serve to handle incoming connections.
// Accepted connections are configured to enable TCP keep-alives.
func (me *Server) ListenAndServe() error {
	for i, loc := range me.CFG.Locations {
		if loc.Pattern == "" {
			loc.Pattern = "/"
		}
		if loc.Handler == "" {
			loc.Handler = "rtmp-live"
		}

		h := NewHandler(me, &me.CFG.Locations[i], me.factory)
		if h == nil {
			me.logger.Warnf("Handler \"%s\" not registered", loc.Handler)
			continue
		}

		me.Mux.Handle(loc.Pattern, h)
	}

	me.logger.Infof("Listening on port %d", me.CFG.Port)

	l, err := net.Listen("tcp", fmt.Sprintf(":%d", me.CFG.Port))
	if err != nil {
		me.logger.Errorf("Failed to listen on port %d", me.CFG.Port)
		return err
	}

	return me.Serve(new(TCPKeepAliveListener).Init(l, time.Duration(me.CFG.MaxIdleTime)*time.Second))
}

// Serve accepts incoming connections on the Listener l, creating a new service goroutine for each.
func (me *Server) Serve(l net.Listener) error {
	defer l.Close()

	d := 5 * time.Millisecond // How long to sleep on accept failure
	m := 1 * time.Second

	for {
		c, err := l.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				me.logger.Warnf("Accept error: %v; retrying in %dms", err, d)

				time.Sleep(d)

				if d *= 2; d > m {
					d = m
				}

				continue
			}

			return err
		}

		// TODO: whitelist & blacklist

		nc := new(NetConnection).Init(c, me, me.logger, me.factory)
		go nc.serve()
	}
}
