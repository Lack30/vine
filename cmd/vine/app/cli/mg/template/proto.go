package template

var (
	ProtoType = `syntax = "proto3";
package {{.Group}}{{.Version}};

option go_package = "{{.Dir}}/proto/apis/{{.Group}}/{{.Version}};{{.Group}}{{.Version}}";

// +gen:runtime={{.Group}}/{{.Version}}
message {{title .Name}}Message {
	string name = 1;
}
`

	ProtoSRV = `syntax = "proto3";

package {{.Group}}{{.Version}};

option go_package = "{{.Dir}}/proto/service/{{.Group}}/{{.Version}};{{.Group}}{{.Version}}";

// +gen:openapi
service {{title .Name}}Service {
	// +gen:post=/{{.Group}}/{{.Version}}/{{.Name}}/Call
	rpc Call(Request) returns (Response) {}
	rpc Stream(StreamingRequest) returns (stream StreamingResponse) {}
	rpc PingPong(stream Ping) returns (stream Pong) {}
}

message Request {
    // +gen:required
	string name = 1;
}

message Response {
	string msg = 1;
}

message StreamingRequest {
	int64 count = 1;
}

message StreamingResponse {
	int64 count = 1;
}

message Ping {
	int64 stroke = 1;
}

message Pong {
	int64 stroke = 1;
}
`

	ProtoNew = `syntax = "proto3";

package {{.Group}}{{.Version}};

option go_package = "{{.Dir}}/proto/service/{{.Group}}/{{.Version}};{{.Group}}{{.Version}}";

// +gen:openapi
service {{title .Name}}Service {
	// +gen:post=/{{.Group}}/{{.Version}}/{{.Name}}/Call
	rpc {{title .Name}}Call({{title .Name}}Request) returns ({{title .Name}}Response) {}
}

message {{title .Name}}Request {
    // +gen:required
	string name = 1;
}

message {{title .Name}}Response {
	string msg = 1;
}
`
)
