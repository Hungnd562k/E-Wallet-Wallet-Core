# 02 — Kiểu dữ liệu

Bài tập: [`learn/02_types`](../../learn/02_types/).

## So sánh nhanh

| C# | Go | Ghi chú |
|---|---|---|
| `class` (reference type) | `struct` là **value type** | Gán `b := a` sẽ copy toàn bộ struct. Muốn chia sẻ thì dùng con trỏ `*T` |
| `ref` / reference | `*T`, `&x` | Không có pointer arithmetic, an toàn như reference trong C# |
| `List<T>` | slice `[]T` | Một header nhỏ trỏ vào mảng chung. `append` có thể cấp phát mảng mới |
| `T[]` cố định | array `[N]T` | Ít dùng trực tiếp |
| `Dictionary<K,V>` | `map[K]V` | Đọc key không tồn tại thì trả zero value. Ghi vào `nil` map thì **panic** |
| `enum Status { A, B }` | `type Status string` + `const` | Hoặc `int` + `iota`. Không có kiểm tra giá trị hợp lệ như enum |
| `record` / value object | `type VND int64` (defined type) | Kiểu mới, không tự trộn với `int64` được |
| `null` | `nil` | Chỉ dành cho pointer, slice, map, chan, func, interface. `string` không bao giờ nil |
| `int?` / `string?` | `*int64`, `*string` | Đây là cách sqlc biểu diễn cột NULL |
| `<T> where T : ...` | `func F[T Constraint](...)` | Constraint là một interface, `~int64` nghĩa là "mọi kiểu có underlying type là int64" |
| `object` | `any` (= `interface{}`) | |
| `decimal` cho tiền | `int64` theo đơn vị nhỏ nhất | VND không có phần lẻ (ADR-09). **Không bao giờ** dùng `float64` cho tiền |

## Pointer receiver hay value receiver?

```go
func (w *Wallet) Freeze()        // pointer receiver: sửa w, hoặc struct lớn
func (w Wallet)  IsActive() bool // value receiver: chỉ đọc, struct nhỏ
```

Quy tắc thực dụng: nếu **một** method cần pointer receiver, cho **tất cả** các method của kiểu đó dùng pointer receiver cho nhất quán. Các service trong repo (`*ledger.Service`, `*payment.Service`) đều được dùng qua con trỏ.

## Struct tag

```go
ID string `json:"payment_id"`
```

Chuỗi trong backtick là metadata, được thư viện đọc qua reflection. Nó tương đương `[JsonPropertyName("payment_id")]` trong C#.

## Thấy ở đâu trong repo

- **Defined type làm enum**: `type Direction string` ở [`internal/ledger/domain/domain.go:14`](../../internal/ledger/domain/domain.go#L14), cùng block `const` ở [dòng 25](../../internal/ledger/domain/domain.go#L25).
- **Struct domain**: `Posting` / `Line` ở [`internal/ledger/domain/domain.go:56`](../../internal/ledger/domain/domain.go#L56).
- **Map làm set**: `map[uuid.UUID]struct{}` ở [`domain.go:104`](../../internal/ledger/domain/domain.go#L104). `struct{}` chiếm 0 byte. Đây là cách viết `HashSet<T>` trong Go.
- **Map of slices** biểu diễn bảng chuyển trạng thái: [`internal/wallet/domain/domain.go:16`](../../internal/wallet/domain/domain.go#L16).
- **Type alias** (`=`) để re-export kiểu ra API public: [`internal/ledger/service.go:26`](../../internal/ledger/service.go#L26). `type Posting = domain.Posting` là **cùng một kiểu**. Ngược lại, `type VND int64` (không có `=`) tạo ra một kiểu **mới**.
- **Pointer biểu diễn NULL**: `Description *string`, `CompletedAt *time.Time` trong `Payment` ở [`internal/payment/service.go:46`](../../internal/payment/service.go#L46). `nilIfEmpty` ở [`internal/wallet/service.go:201`](../../internal/wallet/service.go#L201) đổi `""` thành `nil`.
- **Struct tag JSON**: cũng trong `Payment`. Trường không có tag sẽ dùng nguyên tên trường làm key.
- **Struct ẩn danh** để duyệt qua một danh sách nhỏ: [`internal/payment/service.go:238`](../../internal/payment/service.go#L238).
- **Generics**: [`learn/02_types/types.go`](../../learn/02_types/types.go) (`Sum[T Number]`). Repo chính hiện chưa cần generics.

## Bài tập

1. Viết `Filter` trong [`learn/02_types/types.go`](../../learn/02_types/types.go). Sau đó viết lại thành bản generic `Filter[T any](xs []T, keep func(T) bool) []T`.
2. Đọc `TestStructCopySharesSlices`. Giải thích vì sao sửa `b.Tags[0]` làm thay đổi cả `a`, còn sửa `b.ID` thì không.
3. Thử thêm `Suspended` vào giữa `Active` và `Frozen` trong block `const` của `Status`. Giá trị số của `Frozen` và `Closed` thay đổi thế nào (gợi ý: `iota`)? Nếu DB đã lưu các con số cũ thì chuyện gì xảy ra? Đây là lý do repo dùng enum dạng `string` khi lưu xuống DB.
