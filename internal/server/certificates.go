package server

import (
	"errors"
	"net/http"
	"net/mail"
	"prismflow/internal/core"
	"prismflow/internal/model"
	"slices"
	"strings"
	"time"
)

type Job struct {
	CertificateID string    `json:"certificateId,omitempty"`
	Operation     string    `json:"operation,omitempty"`
	UpdatedAt     time.Time `json:"updatedAt,omitempty"`
	ID            string    `json:"id"`
	Domain        string    `json:"domain"`
	Status        string    `json:"status"`
	Message       string    `json:"message"`
	CreatedAt     time.Time `json:"createdAt"`
}

func (s *Server) importCertificate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID     string `json:"id"`
		Domain string `json:"domain"`
		PEM    string `json:"pem"`
		Key    string `json:"key"`
	}
	if !decode(w, r, &body) {
		return
	}
	cert, err := core.ParseCertificate(body.PEM, body.Key)
	if err == nil {
		err = core.CertificateCovers(cert, body.Domain)
	}
	if err != nil {
		fail(w, 400, errors.New("证书无效、尚未生效或与域名不匹配"))
		return
	}
	c := model.Certificate{ID: model.ID(), Domain: body.Domain, PEM: body.PEM, Key: body.Key, ExpiresAt: cert.NotAfter, Method: "import"}
	err = s.mutate(func(st *model.State) error {
		if body.ID != "" {
			for i := range st.Certificates {
				if st.Certificates[i].ID == body.ID {
					if st.Certificates[i].Method != "import" {
						return errors.New("自动或系统证书不能通过上传替换，请使用续期或重新读取")
					}
					c.ID = body.ID
					st.Certificates[i] = c
					return nil
				}
			}
			return errors.New("待替换的证书不存在")
		}
		if len(st.Certificates) >= 30 {
			return errors.New("最多保存 30 张证书")
		}
		st.Certificates = append(st.Certificates, c)
		return nil
	})
	logID := c.ID
	if body.ID != "" {
		logID = body.ID
	}
	s.recordCertificateResult(logID, c.Domain, "导入证书", err)
	mutationResult(w, err)
}
func (s *Server) issueCertificate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Domain    string `json:"domain"`
		Email     string `json:"email"`
		Method    string `json:"method"`
		DNSSecret string `json:"dnsSecret"`
		Staging   bool   `json:"staging"`
		Engine    string `json:"engine"`
		Agree     bool   `json:"agree"`
	}
	if !decode(w, r, &body) {
		return
	}
	_, emailErr := mail.ParseAddress(body.Email)
	if !body.Agree || emailErr != nil || !core.ValidCertificateDomain(body.Domain) || strings.HasPrefix(body.Domain, "*.") && body.Method != "cloudflare" || !slices.Contains([]string{"http", "cloudflare"}, body.Method) {
		fail(w, 400, errors.New("请填写合法域名、邮箱、验证方式，并同意 ACME 服务条款"))
		return
	}
	if len(s.State().Certificates) >= 30 {
		fail(w, 400, errors.New("最多保存 30 张证书"))
		return
	}
	if body.Engine != "" && body.Engine != "builtin" && body.Engine != "acmesh" {
		fail(w, 400, errors.New("证书引擎不支持"))
		return
	}
	c := model.Certificate{ID: model.ID(), Domain: body.Domain, Email: body.Email, Method: body.Method, DNSSecret: body.DNSSecret, Staging: body.Staging, Renew: true, Engine: body.Engine}
	job, err := s.startIssue(c)
	if err != nil {
		fail(w, 409, err)
		return
	}
	respond(w, 202, job)
}
func (s *Server) renewCertificate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Production bool `json:"production"`
	}
	if r.ContentLength > 0 && !decode(w, r, &body) {
		return
	}
	for _, c := range s.State().Certificates {
		if c.ID == r.PathValue("id") {
			if c.Method == "path" {
				err := s.mutate(func(st *model.State) error { return nil })
				s.recordCertificateResult(c.ID, c.Domain, "重新读取系统证书", err)
				mutationResult(w, err)
				return
			}
			if c.Method == "import" {
				fail(w, 400, errors.New("导入证书需要重新导入"))
				return
			}
			if body.Production {
				c.Staging = false
			}
			job, err := s.startIssue(c)
			if err != nil {
				fail(w, 409, err)
			} else {
				respond(w, 202, job)
			}
			return
		}
	}
	http.NotFound(w, r)
}
func (s *Server) startIssue(c model.Certificate) (Job, error) {
	operation := "申请证书"
	for _, existing := range s.State().Certificates {
		if existing.ID == c.ID {
			operation = "续期证书"
			break
		}
	}
	s.jobsMu.Lock()
	for _, j := range s.jobs {
		if j.Status == "running" {
			s.jobsMu.Unlock()
			return Job{}, errors.New("已有证书任务正在进行")
		}
	}
	job := Job{ID: model.ID(), CertificateID: c.ID, Operation: operation, Domain: c.Domain, Status: "running", Message: "正在验证域名并申请证书", CreatedAt: time.Now().UTC()}
	s.jobs[job.ID] = job
	if err := s.saveCertificateJobsLocked(); err != nil {
		delete(s.jobs, job.ID)
		s.jobsMu.Unlock()
		return Job{}, err
	}
	s.jobsMu.Unlock()
	go func(job Job) {
		issued, err := s.Certificates.Issue(c)
		if err == nil {
			err = s.mutate(func(st *model.State) error {
				for i := range st.Certificates {
					if st.Certificates[i].ID == c.ID {
						st.Certificates[i] = issued
						return nil
					}
				}
				st.Certificates = append(st.Certificates, issued)
				return nil
			})
		}
		job.Status = "done"
		job.Message = "证书已保存并自动应用"
		if err != nil {
			job.Status = "failed"
			job.Message = err.Error()
			_ = s.mutate(func(st *model.State) error {
				for i := range st.Certificates {
					if st.Certificates[i].ID == c.ID {
						st.Certificates[i].LastError = job.Message
					}
				}
				return nil
			})
		}
		job.UpdatedAt = time.Now().UTC()
		s.jobsMu.Lock()
		s.jobs[job.ID] = job
		s.persistCertificateJobsLocked()
		s.jobsMu.Unlock()
		s.Store.Event("证书任务：" + job.Status)
	}(job)
	return job, nil
}
func (s *Server) deleteCertificate(w http.ResponseWriter, r *http.Request) {
	var certificate model.Certificate
	for _, c := range s.State().Certificates {
		if c.ID == r.PathValue("id") {
			certificate = c
			break
		}
	}
	err := s.mutate(func(st *model.State) error {
		for _, e := range st.Endpoints {
			if e.Security == "tls" && !e.Native && !e.ExternalTLS() && e.CertificateID == r.PathValue("id") {
				return errors.New("证书仍被入口引用")
			}
		}
		n := len(st.Certificates)
		if st.DefaultCertificateID == r.PathValue("id") {
			st.DefaultCertificateID = ""
		}
		st.Certificates = slices.DeleteFunc(st.Certificates, func(c model.Certificate) bool { return c.ID == r.PathValue("id") })
		if len(st.Certificates) == n {
			return errors.New("证书不存在")
		}
		return nil
	})
	if certificate.ID != "" {
		s.recordCertificateResult(certificate.ID, certificate.Domain, "删除证书关联", err)
	}
	mutationResult(w, err)
}
func (s *Server) getJobs(w http.ResponseWriter, r *http.Request) {
	s.jobsMu.Lock()
	defer s.jobsMu.Unlock()
	jobs := []Job{}
	for _, j := range s.jobs {
		jobs = append(jobs, j)
	}
	respond(w, 200, jobs)
}
func (s *Server) renewLoop() {
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()
	attempted := map[string]time.Time{}
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			for _, c := range s.State().Certificates {
				if c.Renew && c.Method != "import" && c.Method != "path" && time.Until(c.ExpiresAt) < 30*24*time.Hour && time.Since(attempted[c.ID]) > 12*time.Hour {
					if _, err := s.startIssue(c); err == nil {
						attempted[c.ID] = time.Now()
					}
					break
				}
			}
		}
	}
}
