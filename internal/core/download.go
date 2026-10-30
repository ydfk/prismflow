package core

import (
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"prismflow/internal/model"
	"runtime"
	"time"
)

var coreAssets = map[string]struct{ Name, Checksum string }{
	"linux/amd64":   {"mihomo-linux-amd64-compatible-v1.19.33.gz", "386508bfb1c72c43b6a0eda73708ecdce82546581b0ebcb16366c53757c622b6"},
	"linux/arm64":   {"mihomo-linux-arm64-v1.19.33.gz", "ba408b317080ee265c46cb923a50018c1b82cbd708551cdd46a1e3e307396187"},
	"windows/amd64": {"mihomo-windows-amd64-compatible-v1.19.33.zip", "7aaf636a4f2172dcd8ec34ac4f52797b059195e5afd6409508c963e545f8864d"},
}

func Download(ctx context.Context, target string) error {
	return InstallArchive(ctx, target, "")
}
func InstallArchive(ctx context.Context, target, archive string) error {
	asset, ok := coreAssets[runtime.GOOS+"/"+runtime.GOARCH]
	if !ok {
		return errors.New("内核自动下载仅支持 Linux amd64/arm64 和 Windows amd64 开发验证")
	}
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(target), "mihomo-download-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	var source io.ReadCloser
	if archive != "" {
		source, err = os.Open(archive)
		if err != nil {
			return err
		}
	} else {
		req, e := http.NewRequestWithContext(ctx, "GET", "https://github.com/MetaCubeX/mihomo/releases/download/"+model.CoreVersion+"/"+asset.Name, nil)
		if e != nil {
			return e
		}
		client := http.Client{Timeout: 5 * time.Minute}
		res, e := client.Do(req)
		if e != nil {
			return e
		}
		if res.StatusCode != 200 {
			res.Body.Close()
			return fmt.Errorf("内核下载失败：HTTP %d", res.StatusCode)
		}
		source = res.Body
	}
	defer source.Close()
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(source, 100<<20+1))
	if err != nil {
		return err
	}
	if n > 100<<20 || hex.EncodeToString(h.Sum(nil)) != asset.Checksum {
		return errors.New("内核 SHA-256 校验失败")
	}
	if _, err = f.Seek(0, 0); err != nil {
		return err
	}
	var input io.ReadCloser
	if runtime.GOOS == "windows" {
		z, e := zip.NewReader(f, n)
		if e != nil {
			return e
		}
		for _, entry := range z.File {
			if filepath.Ext(entry.Name) == ".exe" {
				input, err = entry.Open()
				break
			}
		}
		if input == nil {
			return errors.New("内核压缩包中没有可执行文件")
		}
	} else {
		input, err = gzip.NewReader(f)
	}
	if err != nil {
		return err
	}
	defer input.Close()
	out, err := os.CreateTemp(filepath.Dir(target), "mihomo-bin-*")
	if err != nil {
		return err
	}
	path := out.Name()
	defer os.Remove(path)
	n, err = io.Copy(out, io.LimitReader(input, 200<<20+1))
	if err == nil {
		err = out.Chmod(0755)
	}
	closeErr := out.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if n > 200<<20 {
		return errors.New("解压后内核过大")
	}
	if _, err = os.Stat(target); err == nil {
		backup := target + ".previous"
		_ = os.Remove(backup)
		if err = os.Rename(target, backup); err != nil {
			return err
		}
		if err = os.Rename(path, target); err != nil {
			_ = os.Rename(backup, target)
			return err
		}
		return nil
	}
	return os.Rename(path, target)
}
