package server

import (
	"errors"
	"net/http"
	"prismflow/internal/model"
	"slices"
)

type endpointInput struct {
	model.Endpoint
	UserIDs *[]string `json:"userIds"`
}

func assignEndpoint(st *model.State, e model.Endpoint, ids *[]string) error {
	if e.Native {
		if ids != nil && len(*ids) > 0 {
			return errors.New("原生监听器自行管理认证，不能分配订阅用户")
		}
		for i := range st.Users {
			st.Users[i].EndpointIDs = slices.DeleteFunc(st.Users[i].EndpointIDs, func(id string) bool { return id == e.ID })
		}
		return nil
	}
	if ids == nil {
		return nil
	}
	for _, id := range *ids {
		if !slices.ContainsFunc(st.Users, func(u model.User) bool { return u.ID == id }) {
			return errors.New("用户不存在")
		}
	}
	for i := range st.Users {
		st.Users[i].EndpointIDs = slices.DeleteFunc(st.Users[i].EndpointIDs, func(id string) bool { return id == e.ID })
		if slices.Contains(*ids, st.Users[i].ID) {
			st.Users[i].EndpointIDs = append(st.Users[i].EndpointIDs, e.ID)
		}
	}
	return nil
}
func (s *Server) endpointDetails(w http.ResponseWriter, r *http.Request) {
	for _, e := range s.State().Endpoints {
		if e.ID == r.PathValue("id") {
			e.PrivateKey, e.Password = "", ""
			w.Header().Set("Cache-Control", "no-store")
			respond(w, 200, e)
			return
		}
	}
	fail(w, 404, errors.New("入口不存在"))
}
