package template

var (
	MainSRV = `package main

import (
	"{{.Dir}}/handler"
	pb "{{.Dir}}/proto"

	"github.com/lack-io/vine/service"
	"github.com/lack-io/vine/service/logger"
)

func main() {
	// Create service
	srv := service.New(
		service.Name("{{lower .Alias}}"),
		service.Version("latest"),
	)

	// Register handler
	pb.Register{{title .Alias}}Handler(srv.Server(), new(handler.{{title .Alias}}))

	// Run service
	if err := srv.Run(); err != nil {
		logger.Fatal(err)
	}
}
`
)
