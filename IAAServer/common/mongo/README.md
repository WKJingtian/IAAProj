# common/mongo

High-concurrency MongoDB client bootstrap for server modules.

## Quick Start

```go
package main

import (
	"context"
	"log"

	cmongo "common/mongo"
)

func main() {
	_, err := cmongo.InitFromJSON(context.Background(), "mongo_config.json")
	if err != nil {
		log.Fatalf("init mongo failed: %v", err)
	}

	db, err := cmongo.Database()
	if err != nil {
		log.Fatalf("get db failed: %v", err)
	}

	_ = db
}
```

## Config Template

Copy from `config.example.json` and adjust values for your environment.
If authentication is required, fill both `user` and `pwd` (they must be provided together).
