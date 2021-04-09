// MIT License
//
// Copyright (c) 2020 Lack
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package callbacks

import (
	"github.com/lack-io/vine/service/dao"
)

func BeginTransaction(db *dao.DB) {
	if !db.Options.SkipDefaultTransaction {
		if tx := db.Begin(); tx.Error == nil {
			db.Statement.ConnPool = tx.Statement.ConnPool
			db.InstanceSet("dao:started_transaction", true)
		} else if tx.Error == dao.ErrInvalidTransaction {
			tx.Error = nil
		}
	}
}

func CommitOrRollbackTransaction(db *dao.DB) {
	if !db.Options.SkipDefaultTransaction {
		if _, ok := db.InstanceGet("dao:started_transaction"); ok {
			if db.Error == nil {
				db.Commit()
			} else {
				db.Rollback()
			}
			db.Statement.ConnPool = db.ConnPool
		}
	}
}
