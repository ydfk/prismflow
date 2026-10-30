package main

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"prismflow/internal/core"
	"prismflow/internal/server"
	"runtime"
	"syscall"
	"time"
)

var version = "dev"

func env(key, fallback string) string {
	if v := os.Getenv("PRISMFLOW_" + key); v != "" {
		return v
	}
	return fallback
}
func main() {
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}
func run() error {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Println("PrismFlow", version)
		return nil
	}
	if len(os.Args) > 1 && os.Args[1] == "install-core" {
		flags := flag.NewFlagSet("install-core", flag.ContinueOnError)
		target := flags.String("output", "./mihomo", "安装目标路径")
		archive := flags.String("archive", "", "可选的官方压缩包路径，仍校验锁定的 SHA-256")
		if err := flags.Parse(os.Args[2:]); err != nil {
			return err
		}
		return core.InstallArchive(context.Background(), *target, *archive)
	}
	data := flag.String("data", env("DATA", "./data"), "数据目录")
	admin := flag.String("listen", env("LISTEN", "127.0.0.1:8080"), "管理界面监听地址")
	public := flag.String("public-listen", env("PUBLIC_LISTEN", "127.0.0.1:8081"), "网站和订阅监听地址")
	controller := flag.String("controller", env("CONTROLLER", "127.0.0.1:9097"), "Mihomo 控制器，仅允许回环地址")
	binaryDefault := "mihomo"
	if runtime.GOOS == "windows" {
		binaryDefault = "mihomo.exe"
	}
	binary := flag.String("core", env("CORE", binaryDefault), "Mihomo 可执行文件路径")
	origin := flag.String("origin", env("ORIGIN", ""), "反向代理后的管理界面源地址")
	tlsCert := flag.String("tls-cert", env("TLS_CERT", ""), "管理 HTTPS 证书路径")
	tlsKey := flag.String("tls-key", env("TLS_KEY", ""), "管理 HTTPS 私钥路径")
	flag.Parse()
	host, _, err := net.SplitHostPort(*controller)
	if err != nil || !net.ParseIP(host).IsLoopback() {
		return errors.New("控制器必须监听回环 IP")
	}
	if (*tlsCert == "") != (*tlsKey == "") {
		return errors.New("TLS 证书与私钥必须同时提供")
	}
	if *tlsCert != "" {
		if _, err = tls.LoadX509KeyPair(*tlsCert, *tlsKey); err != nil {
			return fmt.Errorf("管理 HTTPS 证书无法读取：%w", err)
		}
	}
	_, publicPort, err := net.SplitHostPort(*public)
	if err != nil {
		return err
	}
	if filepath.IsAbs(*binary) || filepath.Dir(*binary) != "." {
		*binary, err = filepath.Abs(*binary)
		if err != nil {
			return err
		}
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	s, err := server.New(ctx, server.Options{Version: version, InstallMode: env("INSTALL_MODE", "source"), DataDir: *data, CoreBinary: *binary, Controller: *controller, AdminListen: *admin, PublicListen: *public, Origin: *origin, WebsiteURL: "http://127.0.0.1:" + publicPort, SecureCookie: *tlsCert != "" || len(*origin) >= 8 && (*origin)[:8] == "https://"})
	if err != nil {
		return err
	}
	defer s.Close()
	makeHTTP := func(addr string, h http.Handler) *http.Server {
		return &http.Server{Addr: addr, Handler: h, ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 45 * time.Second, WriteTimeout: 90 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32 << 10}
	}
	a := makeHTTP(*admin, s.AdminHandler())
	p := makeHTTP(*public, s.PublicHandler())
	adminListener, err := net.Listen("tcp", *admin)
	if err != nil {
		return err
	}
	publicListener, err := net.Listen("tcp", *public)
	if err != nil {
		adminListener.Close()
		return err
	}
	failures := make(chan error, 2)
	go func() {
		if *tlsCert != "" {
			failures <- a.ServeTLS(adminListener, *tlsCert, *tlsKey)
		} else {
			failures <- a.Serve(adminListener)
		}
	}()
	go func() { failures <- p.Serve(publicListener) }()
	s.Run()
	log.Printf("PrismFlow %s 已启动，管理地址 %s，网站地址 %s", version, *admin, *public)
	log.Print("首次设置令牌位于数据目录的 setup.token 文件")
	select {
	case <-ctx.Done():
	case err = <-failures:
		cancel()
	}
	shutdown, stop := context.WithTimeout(context.Background(), 8*time.Second)
	defer stop()
	_ = a.Shutdown(shutdown)
	_ = p.Shutdown(shutdown)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
