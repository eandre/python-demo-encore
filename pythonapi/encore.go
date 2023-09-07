package pythonapi

import (
	"context"
	"embed"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

//encore:service
type Service struct {
	proxy *httputil.ReverseProxy
}

//encore:api public raw path=/!fallback
func (s *Service) Handler(w http.ResponseWriter, req *http.Request) {
	s.proxy.ServeHTTP(w, req)
}

//go:embed src/*
var src embed.FS

func initService() (*Service, error) {
	tmp, err := os.MkdirTemp("", "")
	if err != nil {
		return nil, err
	}

	err = fs.WalkDir(src, ".", func(path string, d fs.DirEntry, err error) error {
		dst := filepath.Join(tmp, path)
		if d.IsDir() {
			return os.MkdirAll(dst, 0755)
		} else {
			data, _ := src.ReadFile(path)
			return os.WriteFile(dst, data, 0755)
		}
	})
	if err != nil {
		return nil, err
	}

	rootDir := filepath.Join(tmp, "src")
	const port = "18000"

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	if err := startPython(ctx, rootDir, port); err != nil {
		return nil, err
	}

	u := &url.URL{
		Scheme: "http",
		Host:   "localhost:" + port,
		Path:   "/",
	}
	proxy := httputil.NewSingleHostReverseProxy(u)

	return &Service{proxy: proxy}, nil
}

func startPython(ctx context.Context, rootDir, port string) error {
	cmd := exec.Command("uvicorn", "main:app", "--port", port)
	cmd.Dir = rootDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		log.Fatal(err)
	}

	go func() {
		if err := cmd.Wait(); err != nil {
			log.Fatal(err)
		} else {
			log.Fatal("uvicorn exited")
		}
	}()

	return waitForPort(ctx, port)
}

func waitForPort(ctx context.Context, port string) error {
	for ctx.Err() == nil {
		conn, err := net.Dial("tcp", "localhost:"+port)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return ctx.Err()
}
