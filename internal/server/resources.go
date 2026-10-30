package server

import (
	"errors"
	"fmt"
	"net/http"
	"prismflow/internal/core"
	"prismflow/internal/model"
	"slices"
	"strings"
)

func (s *Server) getState(w http.ResponseWriter, r *http.Request) {
	state := s.State()
	applied := s.Applied()
	for i := range state.Endpoints {
		state.Endpoints[i].PrivateKey = ""
		state.Endpoints[i].Password = ""
		state.Endpoints[i].NativeConfig = ""
	}
	for i := range state.Users {
		state.Users[i].UUID = ""
		state.Users[i].Password = ""
		state.Users[i].Token = ""
	}
	for i := range state.Certificates {
		state.Certificates[i].Key = ""
		state.Certificates[i].PEM = ""
		state.Certificates[i].DNSSecret = ""
	}
	respond(w, 200, map[string]any{"state": state, "appliedRevision": applied.Revision, "core": map[string]any{"available": s.Core.Available(), "running": s.Core.Running(), "targetVersion": model.CoreVersion}, "metrics": s.Metrics.Summary(), "traffic": s.Traffic.Sample()})
}
func mutationResult(w http.ResponseWriter, err error) {
	if err != nil {
		fail(w, 400, err)
		return
	}
	respond(w, 200, map[string]bool{"ok": true})
}
func (s *Server) createEndpoint(w http.ResponseWriter, r *http.Request) {
	var input endpointInput
	if !decode(w, r, &input) {
		return
	}
	e := input.Endpoint
	if err := model.InitEndpoint(&e); err != nil {
		fail(w, 500, err)
		return
	}
	mutationResult(w, s.mutate(func(st *model.State) error {
		if err := newEndpointDefaults(&e, *st, s.reserved()); err != nil {
			return err
		}
		st.Endpoints = append(st.Endpoints, e)
		return assignEndpoint(st, e, input.UserIDs)
	}))
}
func (s *Server) updateEndpoint(w http.ResponseWriter, r *http.Request) {
	var input endpointInput
	if !decode(w, r, &input) {
		return
	}
	e := input.Endpoint
	mutationResult(w, s.mutate(func(st *model.State) error {
		for i, old := range st.Endpoints {
			if old.ID == r.PathValue("id") {
				e.ID = old.ID
				e.PrivateKey = old.PrivateKey
				e.PublicKey = old.PublicKey
				e.ShortID = old.ShortID
				e.Password = old.Password
				st.Endpoints[i] = e
				return assignEndpoint(st, e, input.UserIDs)
			}
		}
		return errors.New("入口不存在")
	}))
}
func (s *Server) deleteEndpoint(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	mutationResult(w, s.mutate(func(st *model.State) error {
		found := false
		st.Endpoints = slices.DeleteFunc(st.Endpoints, func(e model.Endpoint) bool {
			if e.ID == id {
				found = true
				return true
			}
			return false
		})
		if !found {
			return errors.New("入口不存在")
		}
		for i := range st.Users {
			st.Users[i].EndpointIDs = slices.DeleteFunc(st.Users[i].EndpointIDs, func(v string) bool { return v == id })
		}
		return nil
	}))
}
func (s *Server) rotateEndpoint(w http.ResponseWriter, r *http.Request) {
	mutationResult(w, s.mutate(func(st *model.State) error {
		for i := range st.Endpoints {
			if st.Endpoints[i].ID == r.PathValue("id") {
				id := st.Endpoints[i].ID
				if err := model.InitEndpoint(&st.Endpoints[i]); err != nil {
					return err
				}
				st.Endpoints[i].ID = id
				return nil
			}
		}
		return errors.New("入口不存在")
	}))
}

type userInput struct {
	Name        string   `json:"name"`
	Enabled     bool     `json:"enabled"`
	EndpointIDs []string `json:"endpointIds"`
}

func (s *Server) createUser(w http.ResponseWriter, r *http.Request) {
	var input userInput
	if !decode(w, r, &input) {
		return
	}
	u := model.NewUser(input.Name)
	u.Enabled = input.Enabled
	u.EndpointIDs = input.EndpointIDs
	mutationResult(w, s.mutate(func(st *model.State) error { st.Users = append(st.Users, u); return nil }))
}
func (s *Server) updateUser(w http.ResponseWriter, r *http.Request) {
	var input userInput
	if !decode(w, r, &input) {
		return
	}
	mutationResult(w, s.mutate(func(st *model.State) error {
		for i := range st.Users {
			if st.Users[i].ID == r.PathValue("id") {
				st.Users[i].Name = input.Name
				st.Users[i].Enabled = input.Enabled
				st.Users[i].EndpointIDs = input.EndpointIDs
				return nil
			}
		}
		return errors.New("用户不存在")
	}))
}
func (s *Server) deleteUser(w http.ResponseWriter, r *http.Request) {
	mutationResult(w, s.mutate(func(st *model.State) error {
		n := len(st.Users)
		st.Users = slices.DeleteFunc(st.Users, func(u model.User) bool { return u.ID == r.PathValue("id") })
		if len(st.Users) == n {
			return errors.New("用户不存在")
		}
		return nil
	}))
}
func (s *Server) rotateUser(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Credentials bool `json:"credentials"`
	}
	if !decode(w, r, &body) {
		return
	}
	mutationResult(w, s.mutate(func(st *model.State) error {
		for i := range st.Users {
			if st.Users[i].ID == r.PathValue("id") {
				st.Users[i].Token = model.Token(32)
				if body.Credentials {
					st.Users[i].UUID = model.UUID()
					st.Users[i].Password = model.Token(24)
				}
				return nil
			}
		}
		return errors.New("用户不存在")
	}))
}
func (s *Server) userSubscription(w http.ResponseWriter, r *http.Request) {
	state := s.State()
	applied := s.Applied()
	for _, u := range state.Users {
		if u.ID != r.PathValue("id") {
			continue
		}
		if !u.Enabled {
			fail(w, 409, errors.New("用户已停用"))
			return
		}
		base := strings.TrimRight(state.PublicURL, "/")
		if base == "" {
			fail(w, 409, errors.New("请先在系统设置中填写公开订阅地址"))
			return
		}
		for _, active := range applied.Users {
			if active.ID == u.ID && active.Enabled {
				links := core.Links(applied, active)
				respond(w, 200, map[string]string{"url": base + "/s/" + u.Token, "linksUrl": base + "/s/" + u.Token + "?format=links", "links": links})
				return
			}
		}
		fail(w, 409, errors.New("用户配置尚未启用，请先安装或启动内核"))
		return
	}
	fail(w, 404, errors.New("用户不存在"))
}
func (s *Server) subscription(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	state := s.State()
	applied := s.Applied()
	var user *model.User
	for _, u := range state.Users {
		if u.Enabled && u.Token == r.PathValue("token") {
			for i := range applied.Users {
				if applied.Users[i].ID == u.ID && applied.Users[i].Enabled {
					user = &applied.Users[i]
					break
				}
			}
			break
		}
	}
	if user == nil {
		http.NotFound(w, r)
		return
	}
	usage := s.Traffic.Sample()
	w.Header().Set("Subscription-Userinfo", usage.Header())
	w.Header().Set("X-PrismFlow-Traffic-Reset-At", fmt.Sprint(usage.ResetAt))
	if !usage.Available {
		w.Header().Set("X-PrismFlow-Traffic-Status", "unavailable")
	} else {
		w.Header().Set("X-PrismFlow-Traffic-Status", "current")
	}
	w.Header().Set("Profile-Update-Interval", "6")
	w.Header().Set("Content-Disposition", `attachment; filename="prismflow.yaml"`)
	if r.URL.Query().Get("format") == "links" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = fmt.Fprint(w, core.Links(applied, *user))
		return
	}
	b, err := core.Subscription(applied, *user)
	if err != nil {
		http.Error(w, err.Error(), 409)
		return
	}
	w.Header().Set("Content-Type", "text/yaml; charset=utf-8")
	_, _ = w.Write(b)
}
func (s *Server) settings(w http.ResponseWriter, r *http.Request) {
	var body struct {
		PublicURL            string                 `json:"publicUrl"`
		RetentionDays        int                    `json:"retentionDays"`
		DefaultHost          *string                `json:"defaultHost"`
		DefaultCertificateID *string                `json:"defaultCertificateId"`
		ApplyHost            bool                   `json:"applyHost"`
		Traffic              *model.TrafficSettings `json:"traffic"`
	}
	if !decode(w, r, &body) {
		return
	}
	mutationResult(w, s.mutate(func(st *model.State) error {
		st.PublicURL = strings.TrimRight(body.PublicURL, "/")
		st.RetentionDays = body.RetentionDays
		if body.Traffic != nil {
			st.Traffic = body.Traffic.Normalized()
		}
		if body.DefaultCertificateID != nil {
			st.DefaultCertificateID = *body.DefaultCertificateID
		}
		if body.DefaultHost != nil {
			st.DefaultHost = strings.TrimSpace(*body.DefaultHost)
			if body.ApplyHost && st.DefaultHost != "" {
				for i := range st.Endpoints {
					st.Endpoints[i].Host = st.DefaultHost
					if st.Endpoints[i].Security == "tls" && st.DefaultCertificateID != "" {
						st.Endpoints[i].CertificateID = st.DefaultCertificateID
						for _, cert := range st.Certificates {
							if cert.ID == st.DefaultCertificateID {
								st.Endpoints[i].ServerName = cert.Domain
							}
						}
					}
				}
			}
		}
		return nil
	}))
}
