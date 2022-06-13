package main

import (
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yuweizzz/sidecar"
)

func main() {
	pid := sidecar.ReadLock()
	// if lock exist
	if pid != 0 {
		alive := sidecar.DetectProcess(pid)
		// if process alive
		if alive {
			fmt.Println("Maybe Server is running and pid is", pid)
			fmt.Println("If Server is not running, remove sidecar-server.lock and retry.")
			panic("Run failed, sidecar-server.lock exist.")
			// if process not alive
		} else {
			sidecar.RemoveLock()
		}
	}
	var cfg *sidecar.Config
	config_file_path := sidecar.DetectFile("conf.toml")
	if config_file_path == "" {
		panic("Run failed, conf.toml not exist.")
	} else {
		cfg = sidecar.ReadConfig(config_file_path)
	}
	log_path := sidecar.CreateDirIfNotExist("log")
	if log_path == "" {
		panic("Create dir for log failed.")
	}
	cert_path := sidecar.CreateDirIfNotExist("certificate")
	if cert_path == "" {
		panic("Create dir for certificate failed.")
	}
	log_fd := sidecar.CreateFileIfNotExist("log/server.log")
	if log_fd == nil {
		log_fd = sidecar.OpenExistFile("log/server.log")
	}
	sidecar.LogRecord(log_fd, "info", "Start Server......")
	sidecar.LogRecord(log_fd, "info", "log location: "+log_path)
	sidecar.LogRecord(log_fd, "info", "certificate location: "+cert_path)
	var pri *rsa.PrivateKey
	var crt *x509.Certificate
	if pri_file_path := sidecar.DetectFile("certificate/sidecar.pri"); pri_file_path == "" {
		pri_fd := sidecar.CreateFileIfNotExist("certificate/sidecar.pri")
		pri = sidecar.GenAndSavePriKey(pri_fd)
		sidecar.LogRecord(log_fd, "info", "Generate new privatekey......")
	} else {
		pri = sidecar.ReadPriKey("certificate/sidecar.pri")
		sidecar.LogRecord(log_fd, "info", "Use exist privatekey......")
	}
	if crt_file_path := sidecar.DetectFile("certificate/sidecar.crt"); crt_file_path == "" {
		crt_fd := sidecar.CreateFileIfNotExist("certificate/sidecar.crt")
		crt = sidecar.GenAndSaveRootCert(crt_fd, pri)
		sidecar.LogRecord(log_fd, "info", "Generate new certificate......")
	} else {
		crt = sidecar.ReadRootCert("certificate/sidecar.crt")
		sidecar.LogRecord(log_fd, "info", "Use exist certificate......")
	}
	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sidecar.HandleHttp(cfg.Server, cfg.ComplexPath, cfg.CustomHeaderName, cfg.CustomHeaderValue, w, r)
		}),
		IdleTimeout:  5 * time.Second,
		TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler)),
		TLSConfig: &tls.Config{
			GetCertificate: func(chi *tls.ClientHelloInfo) (*tls.Certificate, error) {
				return sidecar.GenTLSCert(chi.ServerName, crt, pri)
			},
		},
	}
	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)
	watcher := &sidecar.Listener{Chan: make(chan net.Conn)}
	proxy := &http.Server{
		Addr: ":4396",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sidecar.HandleHttps(watcher, w, r)
		}),
	}
	pid = os.Getpid()
	sidecar.WriteLock(pid)
	fmt.Println("Now Server is running and pid is", pid)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go proxy.ListenAndServe()
	go func() {
		<-sigs
		done <- true
	}()
	sidecar.LogRecord(log_fd, "info", "Awaiting signal......")
	go server.ServeTLS(watcher, "", "")
	<-done
	sidecar.LogRecord(log_fd, "info", "Except signal, exiting......")
	sidecar.RemoveLock()
	defer log_fd.Close()
}
