package httpd

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"sync"

	log "github.com/sirupsen/logrus"
)

/*Conf is the required config to create httpd server*/
type Conf struct {
	Port     uint16
	CertFile string
	KeyFile  string
}

/*Route is the signature of the route handler*/
type Route func(http.ResponseWriter, *http.Request)

/*SimpleServer is a simple http server supporting TLS*/
type SimpleServer interface {
	Port() uint16
	AddRoute(string, Route)
	Start(chan error)
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

func (s *simpleServerImpl) Port() uint16 {
	return s.conf.Port
}

func (s *simpleServerImpl) AddRoute(pattern string, route Route) {
	s.mux.HandleFunc(pattern, route)
}

func (s *simpleServerImpl) Start(errs chan error) {
	certWatcher, err := NewFileWatcher(s.conf.CertFile, func() {
		err := s.load()
		if err != nil {
			log.Error("Error loading certificate %s", err)
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
			log.Error("Error loading certificate %s", err)
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
		log.Info("Certificate and key loaded")
	}
	return err
}

func (s *simpleServerImpl) GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	s.certMutex.RLock()
	defer s.certMutex.RUnlock()
	return s.keyPair, nil
}
