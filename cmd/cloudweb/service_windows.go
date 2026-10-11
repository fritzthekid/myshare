//go:build windows

package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"golang.org/x/sys/windows/svc"
)

// serviceName must match the name under which deploy/windows/install.ps1
// creates the service via New-Service, otherwise svc.IsWindowsService()
// won't reliably detect the service context.
const serviceName = "CloudWeb"

// runService starts the HTTP server and the print folder watcher
// (watcher_windows.go). If the process runs under the control of the
// Service Control Manager (installed via New-Service), handler takes care
// of the start/stop handshake; started interactively (e.g. when testing
// in the VM via "cloudweb.exe" in a console), everything just runs
// blocking directly, without an SCM handshake - convenient for
// development, without having to re-register the service every time.
func runService(st *state, mux http.Handler, addr string) {
	isService, err := svc.IsWindowsService()
	if err != nil {
		log.Fatalf("could not determine service context: %v", err)
	}
	if !isService {
		runInteractive(st, mux, addr)
		return
	}
	if err := svc.Run(serviceName, &serviceHandler{st: st, mux: mux, addr: addr}); err != nil {
		log.Fatalf("service exited with error: %v", err)
	}
}

func runInteractive(st *state, mux http.Handler, addr string) {
	stop := make(chan struct{})
	go startPrintWatcher(st, stop)

	srv := &http.Server{Addr: addr, Handler: mux}
	log.Printf("cloudweb listening on %s (interactive, not a Windows service)", addr)
	log.Fatal(srv.ListenAndServe())
}

type serviceHandler struct {
	st   *state
	mux  http.Handler
	addr string
}

// Execute implements svc.Handler - see
// https://pkg.go.dev/golang.org/x/sys/windows/svc#Handler. The Service
// Control Manager expects an initial status report promptly, after that
// either a repeated new status change or, at the latest before the
// (default) wait time expires, a sign of life.
func (h *serviceHandler) Execute(args []string, r <-chan svc.ChangeRequest, s chan<- svc.Status) (bool, uint32) {
	const accepted = svc.AcceptStop | svc.AcceptShutdown

	s <- svc.Status{State: svc.StartPending}

	stop := make(chan struct{})
	go startPrintWatcher(h.st, stop)

	srv := &http.Server{Addr: h.addr, Handler: h.mux}
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.ListenAndServe() }()

	s <- svc.Status{State: svc.Running, Accepts: accepted}

	for {
		select {
		case err := <-serveErr:
			log.Printf("HTTP server exited: %v", err)
			close(stop)
			s <- svc.Status{State: svc.StopPending}
			return false, 1
		case req := <-r:
			switch req.Cmd {
			case svc.Interrogate:
				s <- req.CurrentStatus
			case svc.Stop, svc.Shutdown:
				s <- svc.Status{State: svc.StopPending}
				close(stop)
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = srv.Shutdown(ctx)
				s <- svc.Status{State: svc.Stopped}
				return false, 0
			}
		}
	}
}
