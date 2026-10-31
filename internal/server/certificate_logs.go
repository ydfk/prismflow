package server

import (
	"log"
	"prismflow/internal/model"
	"sort"
	"time"
)

// 日志只保存操作状态，不记录私钥、账户或 DNS 凭据。
func (s *Server) saveCertificateJobsLocked() error {
	if len(s.jobs) > 200 {
		jobs := []Job{}
		for _, job := range s.jobs {
			if job.Status != "running" {
				jobs = append(jobs, job)
			}
		}
		sort.Slice(jobs, func(i, j int) bool { return jobs[i].CreatedAt.Before(jobs[j].CreatedAt) })
		for _, job := range jobs {
			if len(s.jobs) <= 200 {
				break
			}
			delete(s.jobs, job.ID)
		}
	}
	return s.Store.Set("certificate-jobs", s.jobs)
}
func (s *Server) persistCertificateJobsLocked() {
	if err := s.saveCertificateJobsLocked(); err != nil {
		log.Print("证书日志持久化失败：", err)
	}
}
func (s *Server) recordCertificateResult(id, domain, operation string, err error) {
	job := Job{ID: model.ID(), CertificateID: id, Domain: domain, Operation: operation, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(), Status: "done", Message: operation + "成功"}
	if err != nil {
		job.Status = "failed"
		job.Message = err.Error()
	}
	s.jobsMu.Lock()
	defer s.jobsMu.Unlock()
	s.jobs[job.ID] = job
	s.persistCertificateJobsLocked()
}
