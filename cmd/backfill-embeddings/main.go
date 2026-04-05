// backfill-embeddings 通过 gRPC 调用 AI 服务的 BackfillEmbeddings，触发全量将存量图书写入向量库（后台执行）。
//
// 需：Consul、bookhive.ai 已启动且配置了 OpenAI + Milvus。
//
//	go run ./cmd/backfill-embeddings -consul-addr=127.0.0.1:8500
//
// AI 在 Docker/Podman 内时，Consul 里多为容器 IP，宿主机直连会超时，需 -ai-addr 指到映射端口。
// 本仓库 compose：宿主机 19008→容器 9008；本机 go run ./service/ai 时常为 127.0.0.1:9008。
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/go-micro/plugins/v4/registry/consul"
	consulapi "github.com/hashicorp/consul/api"
	"go-micro.dev/v4"
	"go-micro.dev/v4/client"

	"github.com/qiwang/book-e-commerce-micro/common"
	aiPb "github.com/qiwang/book-e-commerce-micro/proto/ai"
)

func main() {
	consulAddr := flag.String("consul-addr", "127.0.0.1:8500", "Consul address (go-micro registry)")
	aiAddr := flag.String("ai-addr", "127.0.0.1:9008", "bookhive.ai gRPC host:port; direct dial, bypass unreachable Consul node IPs from host. Use -ai-addr= for Consul-only (caller must reach registered address)")
	force := flag.Bool("force", false, "re-embed books that already have a vector")
	maxErrors := flag.Int("max-errors", 0, "embedding failure budget (0 = server default 200)")
	timeout := flag.Duration("timeout", 60*time.Second, "RPC timeout (handler returns quickly; allow slow dial/registry)")
	flag.Parse()

	reg := consul.NewRegistry(consul.Config(&consulapi.Config{Address: *consulAddr}))
	svc := micro.NewService(
		micro.Name("bookhive.backfill-embeddings"),
		micro.Version("1.0.0"),
		micro.Registry(reg),
	)
	// go-micro Init() parses os.Args with urfave/cli; flag.Parse() does not remove flags from os.Args.
	origArgs := os.Args
	if len(origArgs) > 0 {
		os.Args = []string{origArgs[0]}
	}
	svc.Init()
	os.Args = origArgs

	cli := aiPb.NewAIService(common.ServiceAI, svc.Client())
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	var callOpts []client.CallOption
	if *aiAddr != "" {
		callOpts = append(callOpts, client.WithAddress(*aiAddr))
	}

	rsp, err := cli.BackfillEmbeddings(ctx, &aiPb.BackfillEmbeddingsRequest{
		Force:      *force,
		SyncLimit:  0,
		MaxErrors:  int32(*maxErrors),
	}, callOpts...)
	if err != nil {
		log.Fatalf("BackfillEmbeddings: %v", err)
	}

	fmt.Println(rsp.Message)
	if rsp.Background {
		fmt.Println("进度见 AI 服务日志，关键字 [embedding]")
	}
}
