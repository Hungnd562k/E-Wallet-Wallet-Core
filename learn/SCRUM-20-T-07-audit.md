# SCRUM-20 · T-07 Hạ tầng audit log

> Ticket: <https://e-wallet-platform.atlassian.net/browse/SCRUM-20> (Epic E0, nhãn M0, F-22)
> Phân tích dựa trên `development` @ 85493db và docs `E-Wallet` @ master.

## 1. Ticket yêu cầu gì

- **Mục tiêu (NFR-06):** mọi thay đổi tiền và trạng thái đều có audit, ghi **ai làm** (actor), **lý do**, và `request_id`.
- **Schema:** `docs/erd.md` §4.12, bảng `audit.audit_logs`, append-only.
- **Done khi (NFR-04, BR-71):** log **và** audit không chứa CCCD đầy đủ, OTP hay PIN (BR-71 thêm: ảnh, token).

Các quy tắc liên quan trong docs:

| Nguồn | Nội dung |
|---|---|
| architecture §8.2 | Bảng audit append-only; DB role của app chỉ có `INSERT`/`SELECT` trên schema `audit`. |
| architecture §10 Observability | Log, trace **và audit** đều mang `request_id` và `trace_id`. |
| architecture §5.1 | "audit phụ" có thể đi qua outbox; audit chính cho thay đổi tiền thì ghi đồng bộ. |
| erd §4.12 | `reason` bắt buộc với actor `STAFF`; `metadata` không chứa CCCD, ảnh, OTP, PIN, token. |

## 2. Hiện trạng code (đã có gì, còn thiếu gì)

**Đã có:**

- Migration `migrations/0007_audit_init.up.sql`: bảng `audit.audit_logs` đủ cột theo erd, CHECK `actor_type`/`outcome`, CHECK "STAFF phải có reason", 2 index, trigger chặn UPDATE/DELETE/TRUNCATE (`platform.forbid_mutation`).
- Log redaction: `internal/platform/observability/log.go`, `isSensitive` che theo **tên key** (`pin`, `otp`, `token`, `cccd`…), có `log_test.go`.
- `request_id`: middleware `httpx.RequestID` đặt vào ctx, đọc bằng `observability.RequestID(ctx)`.
- Payment và wallet đã ghi `*_history` (có actor, request_id), nhưng đó là lịch sử trạng thái, **không phải** audit.

**Còn thiếu:**

1. `internal/audit/service.go` mới chỉ là stub `// TODO: implement`.
2. Chưa có `internal/audit/store/queries.sql`, chưa có entry `sqlc.yaml` cho audit.
3. Không module nào gọi audit; archtest `allowed` chưa có cạnh `payment → audit`, `wallet → audit`.
4. Chưa có `trace_id` ở đâu cả (chưa có OpenTelemetry), trong khi cột `trace_id` là `NOT NULL`.
5. Redaction chỉ theo tên key. Một số CCCD nằm trong key tên lạ (`id_number`, `note`, `description`) sẽ lọt.
6. DB chỉ có một user `wallet` (owner), chưa có role `wallet_api` chỉ `INSERT`/`SELECT`. Trigger đã chặn sửa/xóa nên tính append-only vẫn được đảm bảo.

## 3. Các subtask

Thứ tự đề xuất: 1 → 2 → 3 → 4 → 5 → 6. Subtask 7 làm sau hoặc tách ticket.

### ST-1. Module `audit`: domain + store (sqlc)

**Làm gì:**

- `internal/audit/domain/entry.go`: kiểu `Entry` (ActorType, ActorID, Action, ResourceType, ResourceID, Outcome, Reason, IP, UserAgent, Metadata `map[string]any`) + hằng `ActorCustomer/Staff/Merchant/System`, `OutcomeSuccess/Denied/Error`.
- `Entry.Validate()`: action/resource không rỗng; actor_type, outcome hợp lệ; `STAFF` thì `Reason` bắt buộc (trùng CHECK của DB, nhưng báo lỗi sớm và rõ hơn).
- Hằng tên action, dạng `<resource>.<verb>`: `payment.transfer`, `wallet.status.change`… Đặt trong `domain` để không gõ chuỗi tay ở nhiều chỗ.
- `internal/audit/store/queries.sql`: một câu `-- name: InsertAuditLog :exec` (chỉ INSERT, không có UPDATE/DELETE). Có thể thêm `ListByResource` để test và cho back-office sau này.
- `sqlc.yaml`: entry mới, `schema: [0001_platform_init.up.sql, 0007_audit_init.up.sql]`. Chạy `make sqlc`.

**Xong khi:** `go build ./...` qua, unit test `Validate` có case STAFF thiếu reason.

> C# so sánh: `Entry` giống một record DTO, `Validate()` giống FluentValidation nhưng viết tay. Package `store` do sqlc sinh ra, tương tự Dapper + code-gen, không sửa tay.

### ST-2. Che dữ liệu nhạy cảm trong `metadata` (phần lõi của "Done khi")

**Làm gì:**

- Tách logic `isSensitive` ra để dùng chung. Cách gọn nhất: export `observability.IsSensitiveKey(key string) bool` (platform được dùng bởi mọi module, không vi phạm archtest).
- Viết `audit.sanitize(metadata)`: duyệt **đệ quy** map/slice, key nhạy cảm thì thay bằng `[REDACTED]`.
- Thêm che theo **giá trị**: chuỗi 12 chữ số liên tiếp (CCCD) thì mask còn 4 số cuối, ví dụ `********1234`. Áp dụng cho cả log (thêm vào `redact` trong `log.go`) để chặn CCCD nằm trong key tên lạ.
- `Record` luôn gọi sanitize trước khi INSERT, caller không cần nhớ.

**Xong khi:** unit test bảng (table-driven) cho: `pin`, `otp`, `refresh_token`, `cccd_number` lồng 2 tầng, slice chứa map, chuỗi có 12 số trong `description`, và case không bị che nhầm (`shipping`, số tiền `1000000`).

> Lưu ý: số tiền VND có thể dài 12 chữ số (`100000000000`). Chỉ áp mask theo giá trị với kiểu `string`, không áp cho số `int64`.

### ST-3. API `audit.Service.Record` + context (actor, request_id, trace_id, ip)

**Làm gì:**

- `service.go` (API công khai duy nhất): `New(pool, clock)`, `Record(ctx, Entry) error`, re-export kiểu bằng type alias (`type Entry = domain.Entry`).
- `Record` **bắt buộc chạy trong tx** (giống `outbox.Enqueue`, `ledger.Post`): dùng `db.Conn(ctx, pool)` và trả lỗi nếu ctx không có tx. Lý do: audit của thay đổi tiền phải commit/rollback cùng giao dịch.
- Tự điền: `id.New()`, `occurred_at = clock.Now()`, `request_id = observability.RequestID(ctx)`.
- **trace_id:** cột đang `NOT NULL` mà chưa có tracing. Đề xuất: middleware `httpx.RequestID` đọc header `traceparent` (W3C) nếu có, lưu trace_id vào ctx (`observability.WithTraceID`), không có thì để chuỗi rỗng `""`. Không sửa migration đã chạy.
- **ip / user_agent:** middleware nhỏ ở httpx đưa vào ctx (hoặc gom chung một struct `RequestMeta`), audit đọc ra.
- Ghi thêm một `slog.InfoContext(ctx, "audit", ...)` không có metadata, để log và audit tương ứng được qua `request_id`.
- Wiring trong `internal/app/app.go`.

**Xong khi:** gọi `Record` ngoài tx trả lỗi; trong tx thì có một dòng trong `audit.audit_logs` với đủ request_id.

### ST-4. Gắn audit vào chỗ thay đổi tiền/trạng thái + cập nhật kiến trúc

**Làm gì:**

- `payment.transferInTx`: sau khi ghi payment + history, gọi `audit.Record` với actor `CUSTOMER`/`cmd.CustomerID`, action `payment.transfer`, resource `payment`/`txn_…`, outcome `SUCCESS`, metadata `{amount, currency, source_wallet, dest_wallet}` (không có PIN dù request có field `pin`).
- `wallet.transition` (`Activate`, sau này `Freeze`/`Close`): action `wallet.status.change`, `reason` lấy từ tham số, metadata `{from, to}`.
- **Thất bại nghiệp vụ (DENIED/ERROR):** tx bị rollback nên dòng audit trong tx cũng mất. Với các lỗi đáng ghi (ví dụ authz từ chối, sau này PIN sai), ghi audit `DENIED` trong **một tx riêng** sau rollback, giống cách `idempotency.SaveFailure` đang làm. Ở M0 có thể chỉ cần SUCCESS cho transfer; ghi lại TODO cho DENIED tại chỗ chèn authz.
- `internal/archtest/arch_test.go`: thêm `"audit"` vào `allowed["payment"]` và `allowed["wallet"]`. **Đồng thời** sửa `E-Wallet/docs/architecture.md` §4.3 (quy tắc trong CLAUDE.md: hai chỗ phải đổi cùng nhau).
- Actor của worker/job (outbox consumer, seed): `SYSTEM`, `actor_id = NULL`.

**Xong khi:** `make test` (archtest) xanh; chuyển tiền thành công tạo đúng 1 dòng audit.

### ST-5. Integration test (testcontainers)

Thêm `test/integration/audit_test.go`:

1. Chuyển tiền thành công → có 1 dòng audit, `actor_id` = customer, `request_id` = header `X-Request-Id` gửi vào, `outcome = SUCCESS`.
2. Chuyển tiền lỗi nghiệp vụ (số dư không đủ) → không có dòng SUCCESS nào (tx rollback cùng nhau).
3. `UPDATE`/`DELETE` trên `audit.audit_logs` → lỗi từ trigger.
4. Request transfer có `pin: "123456"` → `metadata` của dòng audit không chứa `123456`.
5. Bắt log bằng `observability.NewLogger(&buf, …)`, chạy luồng transfer, assert buffer không chứa PIN/OTP/CCCD mẫu (đây chính là "Done khi" của ticket, viết thành test).
6. (Nếu muốn) chạy cùng `TestConcurrentCrossTransfers`: số dòng audit = số payment thành công.

Chạy: `make test-integration RUN=Audit`.

### ST-6. Rà soát log hiện có

**Làm gì:** grep mọi chỗ `slog.*` và `logger.*` trong `internal/` và `cmd/`, kiểm tra:

- Không log request body, không log nguyên `cmd`/struct có field `PIN`.
- Dùng `*Context` để có `request_id`.
- Nếu có struct chứa dữ liệu nhạy cảm, cài `LogValue()` (interface `slog.LogValuer`) để tự che khi bị log. Tương tự override `ToString()` trong C#.

**Xong khi:** có danh sách đã rà (ghi trong PR), không còn chỗ log thô.

### ST-7. (Tùy chọn / tách ticket) DB role chỉ INSERT/SELECT trên audit

architecture §9 muốn có `wallet_api`, `wallet_worker`, `wallet_migrator`. Hiện mọi thứ chạy bằng user `wallet` là owner. Trigger đã đảm bảo append-only, nên ở M0 có thể để lại. Nếu làm:

- Migration tạo role và `GRANT INSERT, SELECT ON audit.audit_logs`, `REVOKE UPDATE, DELETE, TRUNCATE`.
- Tách `DATABASE_URL` của migrate với của api/worker.
- Lưu ý chuyện `search_path` (`"$user", public`) đã ghi trong CLAUDE.md khi đổi tên user.

Đề xuất: tạo ticket riêng ở E0, không chặn SCRUM-20.

## 4. Quyết định cần chốt trước khi code

| # | Câu hỏi | Đề xuất |
|---|---|---|
| 1 | Audit ghi đồng bộ trong tx hay qua outbox? | **Đồng bộ trong tx** cho thay đổi tiền/trạng thái (không bao giờ có tiền đổi mà thiếu audit). Outbox chỉ dùng cho "audit phụ" sau này. |
| 2 | `trace_id` khi chưa có tracing? | Đọc `traceparent` nếu client gửi, không thì `""`. |
| 3 | Có ghi audit DENIED ở M0? | Chưa, chỉ để TODO tại chỗ chèn authz/PIN (SCRUM-21 T-08 sẽ cần). |
| 4 | Che CCCD theo giá trị? | Có, chỉ với chuỗi 12 chữ số, áp cho cả log và audit. |

## 5. Phụ thuộc với ticket khác

- **SCRUM-21 (T-08 PIN):** PIN sai/khóa sau 5 lần là chỗ đầu tiên cần audit `DENIED`. Làm SCRUM-20 trước thì T-08 chỉ việc gọi `audit.Record`.
- **SCRUM-25 (US-21 chuyển bằng SĐT):** đã revert. Khi làm lại, metadata nên dùng SĐT đã mask.
- **Back-office (sau M0):** `reason` bắt buộc với STAFF đã có ở DB; API đọc audit để sau.
