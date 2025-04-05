# Otter 🦦

> Under construction 🚧

Otter is a lightweight proxy designed to intelligently balance traffic to [Dgraph](https://dgraph.io). It currently supports round-robin or purpose-based balancing between groups of endpoints.

### Features

-  Query
-  Mutation
-  Upsert
-  WebSocket JSON API (`ws://localhost:8081/ws`)

---

### Example WebSocket Payload

```json
{
  "type": "upsert",
  "query": "query { u as var(func: eq(email, \"test@example.com\")) }",
  "mutation": "uid(u) <name> \"Test\" .",
  "cond": "@if(eq(len(u), 1))",
  "commitNow": true
}
```

---

### Run Locally

```bash
export CONFIG_FILE=./manifest/config.yaml
go run cmd/proxy/main.go
```

Set your balancer strategy inside `config.yaml`:

```yaml
balancer_type: purposeful # or round-robin
```

---

###  Roadmap

- [ ] Automatic health checks
- [ ] Support for multiple Balancing strategies
- [ ] Graph model abstraction
- [ ] Become a framework

More purposeful Balancing strategies:
- [x] `round-robin` basic round-robin
- [x] `round-robin-purposeful` with purpose
- [ ] `round-robin-healthy` support
- [ ] `round-robin-on-RW` separate readonly and write only
- [ ] `round-robin-avoid-leaders` avoid leaders
- [ ] `round-robin-leaders-only` leaders only
- [ ] `round-robin-state-based` this will check the state of the Alpha and check memory usage and coroutine count
---