# models-go-memory

> **Fast In-Memory Mock Store Adapter for Unit Testing & Prototyping**

`models-go-memory` provides a zero-dependency in-memory mock database adapter for `models-go-engine`. It allows instant unit testing and standalone dataset execution without requiring a live database server.

---

## 🛠️ Key Exported Functions & Methods Reference

### `MemoryAdapter` ([`memory.go`](./memory.go))

| Function / Method | Signature | Description |
| :--- | :--- | :--- |
| `NewMemoryAdapter` | `() *MemoryAdapter` | Creates a new thread-safe in-memory adapter. |
| `Seed` | `(tableName string, records []map[string]any)` | Seeds initial mock data into memory. |
| `Create` | `(ctx context.Context, ref model.ModelRef, data map[string]any)` | Inserts record into in-memory collection store. |
| `Find` | `(ctx context.Context, ref model.ModelRef, q query.Query)` | Filters and returns in-memory records matching criteria. |

---

## 🚀 Usage Example

```go
package main

import (
	"context"
	"fmt"

	"github.com/SanjayDrop5528/models-go-memory"
)

func main() {
	mem := memory.NewMemoryAdapter()
	mem.Seed("users", []map[string]any{
		{"id": 1, "name": "Alice", "role": "admin"},
		{"id": 2, "name": "Bob", "role": "user"},
	})

	fmt.Println("Memory Adapter initialized successfully.")
}
```
