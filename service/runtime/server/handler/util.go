// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package handler

import (
	"context"

	pb "github.com/lack-io/vine/proto/runtime"
	"github.com/lack-io/vine/service/runtime"
)

func toProto(s *runtime.Service) *pb.Service {
	return &pb.Service{
		Name:     s.Name,
		Version:  s.Version,
		Source:   s.Source,
		Metadata: s.Metadata,
		Status:   int32(s.Status),
	}
}

func toService(s *pb.Service) *runtime.Service {
	// add status to metadata to enable backwards compatability
	md := s.Metadata
	if md == nil {
		md = map[string]string{"status": humanizeStatus(s.Status)}
	} else {
		md["status"] = humanizeStatus(s.Status)
	}

	return &runtime.Service{
		Name:     s.Name,
		Version:  s.Version,
		Source:   s.Source,
		Metadata: md,
		Status:   runtime.ServiceStatus(s.Status),
	}
}

func humanizeStatus(status int32) string {
	switch runtime.ServiceStatus(status) {
	case runtime.Pending:
		return "pending"
	case runtime.Building:
		return "building"
	case runtime.Starting:
		return "starting"
	case runtime.Running:
		return "running"
	case runtime.Stopping:
		return "stopping"
	case runtime.Stopped:
		return "stopped"
	case runtime.Error:
		return "error"
	default:
		return "unknown"
	}
}

func toCreateOptions(ctx context.Context, opts *pb.CreateOptions) []runtime.CreateOption {
	options := []runtime.CreateOption{
		runtime.CreateNamespace(opts.Namespace),
		runtime.CreateEntrypoint(opts.Entrypoint),
	}

	// command options
	if len(opts.Command) > 0 {
		options = append(options, runtime.WithCommand(opts.Command...))
	}

	// args for command
	if len(opts.Args) > 0 {
		options = append(options, runtime.WithArgs(opts.Args...))
	}

	// env options
	if len(opts.Env) > 0 {
		options = append(options, runtime.WithEnv(opts.Env))
	}

	// create specific type of service
	if len(opts.Type) > 0 {
		options = append(options, runtime.CreateType(opts.Type))
	}

	// use specific image
	if len(opts.Image) > 0 {
		options = append(options, runtime.CreateImage(opts.Image))
	}

	// inject the secrets
	for k, v := range opts.Secrets {
		options = append(options, runtime.WithSecret(k, v))
	}

	// mount the volumes
	for name, path := range opts.Volumes {
		options = append(options, runtime.WithVolume(name, path))
	}

	// TODO: output options

	return options
}

func toReadOptions(ctx context.Context, opts *pb.ReadOptions) []runtime.ReadOption {
	options := []runtime.ReadOption{
		runtime.ReadNamespace(opts.Namespace),
	}

	if len(opts.Service) > 0 {
		options = append(options, runtime.ReadService(opts.Service))
	}
	if len(opts.Version) > 0 {
		options = append(options, runtime.ReadVersion(opts.Version))
	}
	if len(opts.Type) > 0 {
		options = append(options, runtime.ReadType(opts.Type))
	}

	return options
}

func toUpdateOptions(ctx context.Context, opts *pb.UpdateOptions) []runtime.UpdateOption {
	return []runtime.UpdateOption{
		runtime.UpdateNamespace(opts.Namespace),
		runtime.UpdateEntrypoint(opts.Entrypoint),
	}
}

func toDeleteOptions(ctx context.Context, opts *pb.DeleteOptions) []runtime.DeleteOption {
	return []runtime.DeleteOption{
		runtime.DeleteNamespace(opts.Namespace),
	}
}

func toLogsOptions(ctx context.Context, opts *pb.LogsOptions) []runtime.LogsOption {
	return []runtime.LogsOption{
		runtime.LogsNamespace(opts.Namespace),
	}
}
