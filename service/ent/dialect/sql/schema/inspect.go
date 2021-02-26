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

package schema

import (
	"context"
	"fmt"

	"github.com/lack-io/vine/service/ent/dialect"
	"github.com/lack-io/vine/service/ent/dialect/sql"
)

// InspectOption allows for managing schema configuration using functional options.
type InspectOption func(inspect *Inspector)

// WithSchema provides a schema (named-database) for reading the tables from.
func WithSchema(schema string) InspectOption {
	return func(m *Inspector) {
		m.schema = schema
	}
}

// An Inspector provides methods for inspecting database tables.
type Inspector struct {
	sqlDialect
	schema string
}

// NewInspect returns an inspector for the given SQL driver.
func NewInspect(d dialect.Driver, opts ...InspectOption) (*Inspector, error) {
	i := &Inspector{}
	for _, opt := range opts {
		opt(i)
	}
	return i, nil
}

// Tables returns the tables in the schema.
func (i *Inspector) Tables(ctx context.Context) ([]*Table, error) {
	names, err := i.tables(ctx)
	if err != nil {
		return nil, err
	}
	tx := dialect.NopTx(i.sqlDialect)
	tables := make([]*Table, 0, len(names))
	for _, name := range names {
		t, err := i.table(ctx, tx, name)
		if err != nil {
			return nil, err
		}
		tables = append(tables, t)
	}
	return tables, nil
}

func (i *Inspector) tables(ctx context.Context) ([]string, error) {
	t, ok := i.sqlDialect.(interface{ tables() sql.Querier })
	if !ok {
		return nil, fmt.Errorf("sql/schema: %q driver does not support inspection", i.Dialect())
	}
	query, args := t.tables().Query()
	var (
		names []string
		rows  = &sql.Rows{}
	)
	if err := i.Query(ctx, query, args, rows); err != nil {
		return nil, fmt.Errorf("mysql: reading table names %w", err)
	}
	defer rows.Close()
	if err := sql.ScanSlice(rows, &names); err != nil {
		return nil, err
	}
	return names, nil
}