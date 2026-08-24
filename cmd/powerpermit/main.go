package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"powerpermit/internal/application"
	"powerpermit/internal/audit"
	"powerpermit/internal/storage"
	webserver "powerpermit/internal/web"
)

func main() {
	addressFlag := flag.String("addr", defaultAddress, "监听地址")
	dataDirFlag := flag.String("data", "data", "事件日志和快照目录")
	selfcheckFlag := flag.Bool("selfcheck", false, "执行完整业务自检后退出")
	flag.Parse()
	address, err := resolveAddress(*addressFlag)
	if err != nil {
		log.Fatal(err)
	}
	dataDir := *dataDirFlag
	if *selfcheckFlag {
		dataDir, err = os.MkdirTemp("", "powerpermit-selfcheck-")
		if err != nil {
			log.Fatal(err)
		}
		defer os.RemoveAll(dataDir)
	}
	repo, err := storage.Open(filepath.Clean(dataDir))
	if err != nil {
		log.Fatal(err)
	}
	service := application.New(repo, audit.New())
	handler := webserver.New(service).Handler()
	server := &http.Server{Addr: address, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}
	if *selfcheckFlag {
		if err := runSelfcheck(server, address); err != nil {
			log.Fatal(err)
		}
		fmt.Println("selfcheck passed: 建档、方案、检查、复核、冻结和许可摘要均通过")
		return
	}
	errChannel := make(chan error, 1)
	go func() { errChannel <- server.ListenAndServe() }()
	log.Printf("临时用电许可工作台已启动：http://%s", address)
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	select {
	case signal := <-signals:
		log.Printf("收到 %s，正在关闭", signal)
	case serveErr := <-errChannel:
		if !errors.Is(serveErr, http.ErrServerClosed) {
			log.Fatal(serveErr)
		}
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("关闭服务失败：%v", err)
	}
}
