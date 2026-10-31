package server

import (
	"context"
	"errors"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"prismflow/internal/certificates"
	"prismflow/internal/core"
	"prismflow/internal/hosttraffic"
	"prismflow/internal/metrics"
	"prismflow/internal/model"
	"prismflow/internal/store"
	"prismflow/internal/website"
	"prismflow/internal/webui"
	"strconv"
	"strings"
	"sync"
)

type Options struct {
	Version, InstallMode                                                           string
	DataDir, CoreBinary, Controller, AdminListen, PublicListen, Origin, WebsiteURL string
	SecureCookie                                                                   bool
}
type Server struct {
	panelUpdates   panelUpdater
	Options        Options
	Store          *store.Store
	Core           *core.Manager
	Certificates   *certificates.Manager
	Metrics        *metrics.Collector
	Traffic        *hosttraffic.Monitor
	checkMu        sync.Mutex
	mu             sync.Mutex
	state, applied model.State
	authMu         sync.Mutex
	sessions       map[string]session
	attempts       map[string]attempt
	setupToken     string
	jobsMu         sync.Mutex
	jobs           map[string]Job
	ctx            context.Context
	installMu      sync.Mutex
	installJob     Job
}

func New(ctx context.Context, o Options) (*Server, error) {
	dir, err := filepath.Abs(o.DataDir)
	if err != nil {
		return nil, err
	}
	o.DataDir = dir
	db, err := store.Open(dir)
	if err != nil {
		return nil, err
	}
	state, err := db.State()
	if err != nil {
		db.DB.Close()
		return nil, err
	}
	var secret string
	if err = db.Get("controller-secret", &secret); err != nil {
		if !isMissing(err) {
			return nil, err
		}
		secret = model.Token(32)
		if err = db.Set("controller-secret", secret); err != nil {
			return nil, err
		}
	}
	s := &Server{Options: o, Store: db, state: state, applied: model.Default(), sessions: map[string]session{}, attempts: map[string]attempt{}, jobs: map[string]Job{}, ctx: ctx}
	if err = db.Get("certificate-jobs", &s.jobs); err != nil && !isMissing(err) {
		db.DB.Close()
		return nil, err
	}
	for id, job := range s.jobs {
		if job.Status == "running" {
			job.Status = "failed"
			job.Message = "服务已重启，上次任务中断，可重新申请或续期"
			s.jobs[id] = job
		}
	}
	if err = db.Get("applied", &s.applied); err != nil && !isMissing(err) {
		return nil, err
	}
	managed := managedCorePath(dir)
	if _, err := os.Stat(managed); err == nil {
		o.CoreBinary = managed
	}
	s.Core = core.NewManager(o.CoreBinary, filepath.Join(dir, "core"), o.Controller, secret)
	s.Certificates = &certificates.Manager{Store: db, Home: filepath.Join(dir, "acmesh")}
	s.Metrics = &metrics.Collector{Store: db, Core: s.Core, State: s.Applied}
	s.Traffic = &hosttraffic.Monitor{Store: db, Settings: func() model.TrafficSettings { return s.State().Traffic }}
	var admin adminAccount
	if err = db.Get("admin", &admin); isMissing(err) {
		tokenPath := filepath.Join(dir, "setup.token")
		b, readErr := os.ReadFile(tokenPath)
		if errors.Is(readErr, os.ErrNotExist) {
			s.setupToken = model.Token(24)
			readErr = os.WriteFile(tokenPath, []byte(s.setupToken), 0600)
		} else {
			s.setupToken = string(b)
		}
		if readErr != nil {
			return nil, readErr
		}
	} else if err != nil {
		return nil, err
	}
	return s, nil
}
func (s *Server) Close() { s.Core.Stop(); _ = s.Store.DB.Close() }
func (s *Server) Applied() model.State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return model.Clone(s.applied)
}
func (s *Server) State() model.State { s.mu.Lock(); defer s.mu.Unlock(); return model.Clone(s.state) }
func (s *Server) Run() {
	// 精简 Docker 镜像不预装内核；下载在后台执行，失败时面板仍可用于重试。
	if s.Options.InstallMode == "docker" && !s.Core.Available() {
		s.startCoreInstall()
	}
	go s.Metrics.Run(s.ctx)
	go s.Traffic.Run(s.ctx)
	go s.renewLoop()
	go s.watchCertificateFiles()
	go func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		applied := model.Clone(s.applied)
		if applied.Revision > 0 && s.Core.Available() {
			b, err := core.Render(applied, s.Options.Controller, s.Core.Secret, s.Options.WebsiteURL)
			if err == nil {
				err = s.Core.Publish(s.ctx, b, func() error { return nil })
			}
			if err != nil {
				s.Store.Event("内核自动启动失败，请在系统设置检查并启动")
			} else {
				s.Store.Event("已启动上次生效的配置")
			}
		}
	}()
}
func (s *Server) reserved() []int {
	result := []int{}
	for _, addr := range []string{s.Options.AdminListen, s.Options.PublicListen, s.Options.Controller} {
		_, port, err := net.SplitHostPort(addr)
		if err == nil {
			n, _ := strconv.Atoi(port)
			result = append(result, n)
		}
	}
	return result
}
func (s *Server) mutate(fn func(*model.State) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := model.Clone(s.state)
	if err := fn(&next); err != nil {
		return err
	}
	return s.saveLocked(s.ctx, next)
}
func (s *Server) AdminHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/prismflow-access", accessMarker("panel"))
	mux.HandleFunc("GET /api/status", s.status)
	mux.HandleFunc("POST /api/setup", s.setup)
	mux.HandleFunc("POST /api/session", s.login)
	api := http.NewServeMux()
	api.HandleFunc("GET /api/updates", s.updateStatus)
	api.HandleFunc("POST /api/updates/check", s.checkUpdate)
	api.HandleFunc("POST /api/updates/install", s.requestUpdate)
	api.HandleFunc("PUT /api/updates/automatic", s.automaticUpdates)
	api.HandleFunc("GET /api/state", s.getState)
	api.HandleFunc("GET /api/protocols", func(w http.ResponseWriter, r *http.Request) { respond(w, 200, model.Protocols) })
	api.HandleFunc("DELETE /api/session", s.logout)
	api.HandleFunc("POST /api/password", s.changePassword)
	api.HandleFunc("GET /api/listener-types", func(w http.ResponseWriter, r *http.Request) { respond(w, 200, model.ListenerTypes) })
	api.HandleFunc("POST /api/endpoints/{id}/check", s.checkEndpoint)
	api.HandleFunc("GET /api/endpoints/{id}", s.endpointDetails)
	api.HandleFunc("POST /api/endpoints", s.createEndpoint)
	api.HandleFunc("PUT /api/endpoints/{id}", s.updateEndpoint)
	api.HandleFunc("DELETE /api/endpoints/{id}", s.deleteEndpoint)
	api.HandleFunc("POST /api/endpoints/{id}/rotate", s.rotateEndpoint)
	api.HandleFunc("POST /api/users", s.createUser)
	api.HandleFunc("PUT /api/users/{id}", s.updateUser)
	api.HandleFunc("DELETE /api/users/{id}", s.deleteUser)
	api.HandleFunc("POST /api/users/{id}/rotate", s.rotateUser)
	api.HandleFunc("GET /api/users/{id}/subscription", s.userSubscription)
	api.HandleFunc("POST /api/core/{action}", s.coreAction)
	api.HandleFunc("POST /api/core/install", s.installCore)
	api.HandleFunc("GET /api/core/install", s.coreInstallStatus)
	api.HandleFunc("POST /api/endpoints/preset", s.createPreset)
	api.HandleFunc("POST /api/ports/random", s.randomPorts)
	api.HandleFunc("POST /api/reality/test", s.testRealityTargets)
	api.HandleFunc("POST /api/certificates/path", s.linkCertificatePath)
	api.HandleFunc("GET /api/certificates/files", s.certificateFiles)
	api.HandleFunc("GET /api/events", s.events)
	api.HandleFunc("GET /api/metrics", s.history)
	api.HandleFunc("GET /api/access/config", s.accessConfig)
	api.HandleFunc("POST /api/access/check", s.checkAccess)
	api.HandleFunc("PUT /api/settings", s.settings)
	api.HandleFunc("POST /api/certificates/import", s.importCertificate)
	api.HandleFunc("POST /api/certificates/issue", s.issueCertificate)
	api.HandleFunc("POST /api/certificates/{id}/renew", s.renewCertificate)
	api.HandleFunc("DELETE /api/certificates/{id}", s.deleteCertificate)
	api.HandleFunc("GET /api/jobs", s.getJobs)
	api.HandleFunc("PUT /api/website", s.updateWebsite)
	api.HandleFunc("POST /api/website/upload", s.uploadWebsite)
	api.HandleFunc("POST /api/backup", s.backup)
	api.HandleFunc("POST /api/restore", s.restore)
	mux.Handle("/api/", s.auth(api))
	assets := webui.Files()
	files := http.FileServer(http.FS(assets))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" && r.Method != "HEAD" {
			http.Error(w, "method not allowed", 405)
			return
		}
		if _, err := fs.Stat(assets, "index.html"); err != nil {
			http.Error(w, "前端尚未构建，请执行 web 下的 npm run build", 503)
			return
		}
		if r.URL.Path == "/" {
			files.ServeHTTP(w, r)
			return
		}
		if _, err := fs.Stat(assets, r.URL.Path[1:]); err != nil {
			http.NotFound(w, r)
			return
		}
		files.ServeHTTP(w, r)
	})
	return s.headers(mux)
}
func (s *Server) PublicHandler() http.Handler {
	mux := http.NewServeMux()
	admin := s.AdminHandler()
	mux.HandleFunc("GET /.well-known/acme-challenge/{token}", s.Certificates.Challenge)
	mux.HandleFunc("GET /.well-known/prismflow-access", accessMarker("public"))
	mux.HandleFunc("GET /s/{token}", s.subscription)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		prefix := s.State().Access.ManagementPath()
		if r.URL.Path == prefix {
			http.Redirect(w, r, prefix+"/", http.StatusTemporaryRedirect)
			return
		}
		if strings.HasPrefix(r.URL.Path, prefix+"/") {
			http.StripPrefix(prefix, admin).ServeHTTP(w, r)
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" {
			http.NotFound(w, r)
			return
		}
		website.Serve(w, r, s.State().Website, filepath.Join(s.Options.DataDir, "sites"))
	})
	return mux
}
func (s *Server) headers(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
		if len(r.URL.Path) >= 5 && r.URL.Path[:5] == "/api/" {
			w.Header().Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}
