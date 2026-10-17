# E-Wallet-Wallet-Core

Core ví: service `wallet-core` (Go) của nền tảng E-Wallet. Kiến trúc nằm ở `../E-Wallet/docs/architecture.md`.

```bash
cp .env.example .env
make up migrate-up seed    # Postgres (cổng 5433) + schema + 2 ví demo
make run-api               # :8080
make run-worker
make test                  # unit + archtest + learn
make test-integration      # cần Docker
make help                  # mọi lệnh
```

- Mới học Go? Bắt đầu từ [`docs/go-map/00-index.md`](docs/go-map/00-index.md).
- Quy ước và kiến trúc code: [`CLAUDE.md`](CLAUDE.md).
