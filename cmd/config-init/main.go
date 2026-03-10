package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	consulapi "github.com/hashicorp/consul/api"
)

func main() {
	consulAddr := flag.String("consul", "127.0.0.1:8500", "Consul address")
	configFile := flag.String("file", "config.yaml", "Path to config YAML file")
	kvKey := flag.String("key", "bookhive/config", "Consul KV key")
	flag.Parse()

	data, err := os.ReadFile(*configFile)
	if err != nil {
		log.Fatalf("failed to read %s: %v", *configFile, err)
	}

	client, err := consulapi.NewClient(&consulapi.Config{Address: *consulAddr})
	if err != nil {
		log.Fatalf("failed to create consul client: %v", err)
	}

	pair := &consulapi.KVPair{
		Key:   *kvKey,
		Value: data,
	}
	_, err = client.KV().Put(pair, nil)
	if err != nil {
		log.Fatalf("failed to write to consul kv: %v", err)
	}

	fmt.Printf("Successfully pushed %s (%d bytes) to Consul KV [%s] at %s\n",
		*configFile, len(data), *kvKey, *consulAddr)
}
