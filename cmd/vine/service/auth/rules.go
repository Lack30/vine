// Copyright 2020 lack
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package auth

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/lack-io/cli"

	pb "github.com/lack-io/vine/proto/auth"
	"github.com/lack-io/vine/proto/errors"
	"github.com/lack-io/vine/util/client"
)

func listRules(ctx *cli.Context) {
	client := rulesFromContext(ctx)

	rsp, err := client.List(context.TODO(), &pb.ListRequest{})
	if err != nil {
		fmt.Printf("Error listing rules: %v\n", err)
		os.Exit(1)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 8, 1, '\t', 0)
	defer w.Flush()

	formatResource := func(r *pb.Resource) string {
		return strings.Join([]string{r.Type, r.Name, r.Endpoint}, ":")
	}

	// sort rules using resource name and priority to keep the list consistent
	sort.Slice(rsp.Rules, func(i, j int) bool {
		resI := formatResource(rsp.Rules[i].Resource) + string(rsp.Rules[i].Priority)
		resJ := formatResource(rsp.Rules[j].Resource) + string(rsp.Rules[j].Priority)
		return sort.StringsAreSorted([]string{resJ, resI})
	})

	fmt.Fprintln(w, strings.Join([]string{"ID", "Scope", "Access", "Resource", "Priority"}, "\t\t"))
	for _, r := range rsp.Rules {
		res := formatResource(r.Resource)
		if r.Scope == "" {
			r.Scope = "<public>"
		}
		fmt.Fprintln(w, strings.Join([]string{r.Id, r.Scope, r.Access.String(), res, fmt.Sprintf("%d", r.Priority)}, "\t\t"))
	}
}

func createRule(ctx *cli.Context) {
	_, err := rulesFromContext(ctx).Create(context.TODO(), &pb.CreateRequest{
		Rule: constructRule(ctx),
	})
	if verr, ok := err.(*errors.Error); ok {
		fmt.Printf("Error: %v\n", verr.Detail)
		return
	} else if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println("Rule created")
}

func deleteRule(ctx *cli.Context) {
	if ctx.Args().Len() != 1 {
		fmt.Println("Expected one argument: ID")
		os.Exit(1)
	}

	_, err := rulesFromContext(ctx).Delete(context.TODO(), &pb.DeleteRequest{
		Id: ctx.Args().First(),
	})
	if verr, ok := err.(*errors.Error); ok {
		fmt.Printf("Error: %v\n", verr.Detail)
		return
	} else if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println("Rule deleted")
}

func constructRule(ctx *cli.Context) *pb.Rule {
	if ctx.Args().Len() != 1 {
		fmt.Println("Too many arguments, expected one argument: ID")
		os.Exit(1)
	}

	var access pb.Access
	switch ctx.String("access") {
	case "granted":
		access = pb.Access_GRANTED
	case "denied":
		access = pb.Access_DENIED
	default:
		fmt.Printf("Invalid access: %v, must be granted or denied\n", ctx.String("access"))
		os.Exit(1)
	}

	resComps := strings.Split(ctx.String("resource"), ":")
	if len(resComps) != 3 {
		fmt.Println("Invalid resource, must be in the format type:name:endpoint")
		os.Exit(1)
	}

	return &pb.Rule{
		Id:       ctx.Args().First(),
		Access:   access,
		Scope:    ctx.String("scope"),
		Priority: int32(ctx.Int("priority")),
		Resource: &pb.Resource{
			Type:     resComps[0],
			Name:     resComps[1],
			Endpoint: resComps[2],
		},
	}
}

func rulesFromContext(ctx *cli.Context) pb.RulesService {
	return pb.NewRulesService("go.vine.auth", client.New(ctx))
}
