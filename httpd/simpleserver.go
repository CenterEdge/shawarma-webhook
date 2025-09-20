package httpd

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"go.uber.org/zap"
)

/*Conf is the required config to create httpd server*/
type Conf struct {
	Port     uint16
	CertFile string
	KeyFile  string
	Logger   *zap.Logger
}

/*Route is the signature of the route handler*/
type Route func(http.ResponseWriter, *http.Request)

/*SimpleServer is a simple http server supporting TLS*/
type SimpleServer interface {
	AddRoute(string, Route)
	Start(chan error)
	StartAndWait() error
	Shutdown()
}

/*NewSimpleServer is a factory function to create an instance of SimpleServer*/
func NewSimpleServer(conf Conf) SimpleServer {
	return &simpleServerImpl{
		conf: conf,
		mux:  http.NewServeMux(),
		server: &http.Server{
			Addr: fmt.Sprintf(":%d", conf.Port),
		},
	}
}

type simpleServerImpl struct {
	conf   Conf
	server *http.Server
	mux    *http.ServeMux

	certMutex sync.RWMutex
	certWatcher *FileWatcher
	keyWatcher *FileWatcher
	keyPair *tls.Certificate
}

func (s *simpleServerImpl) AddRoute(pattern string, route Route) {
	s.mux.HandleFunc(pattern, route)
}

func (s *simpleServerImpl) Start(errs chan error) {
	certWatcher, err := NewFileWatcher(s.conf.CertFile, func() {
		err := s.load()
		if err != nil {
			s.conf.Logger.Error("Error loading certificate", 
				zap.Error(err))
		}
	})
	if err != nil {
		errs <- err
		return
	}
	s.certWatcher = &certWatcher

	keyWatcher, err := NewFileWatcher(s.conf.KeyFile, func() {
		err := s.load()
		if err != nil {
			s.conf.Logger.Error("Error loading key", 
				zap.Error(err))
		}
	})
	if err != nil {
		errs <- err
		return
	}
	s.keyWatcher = &keyWatcher

	// Load initially
	err = s.load()
	if err != nil {
		errs <- err
		return
	}

	s.server.TLSConfig = &tls.Config{GetCertificate: s.GetCertificate}

	s.server.Handler = s.mux
	go func() {
		if err := s.server.ListenAndServeTLS("", ""); err != nil {
			errs <- err
		}
	}()
}

func (s *simpleServerImpl) StartAndWait() error {
	errC := make(chan error, 1)
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	defer func() {
		close(errC)
		close(signalChan)
	}()

	s.conf.Logger.Info("SimpleServer starting to listen",
		zap.Uint16("port", s.conf.Port))

	s.Start(errC)

	// block until an error or signal from os to
	// terminate the process
	var retErr error
	select {
	case err := <-errC:
		retErr = err
	case <-signalChan:
	}

	return retErr
}

func (s *simpleServerImpl) Shutdown() {
	s.server.Shutdown(context.Background())

	if s.certWatcher != nil {
		(*s.certWatcher).Close()
		s.certWatcher = nil
	}

	if s.keyWatcher != nil {
		(*s.keyWatcher).Close()
		s.keyWatcher = nil
	}
}

func (s *simpleServerImpl) load() error {
	keyPair, err := tls.LoadX509KeyPair(s.conf.CertFile, s.conf.KeyFile)
	if err == nil {
		s.certMutex.Lock()
		s.keyPair = &keyPair
		s.certMutex.Unlock()
		s.conf.Logger.Info("Certificate and key loaded")
	}
	return err
}

func (s *simpleServerImpl) GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	s.certMutex.RLock()
	defer s.certMutex.RUnlock()
	return s.keyPair, nil
}
