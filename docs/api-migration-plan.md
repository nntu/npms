# Kế hoạch chuyển API sang typed contract

## 1. Mục tiêu

Giảm lệch giữa Go API, frontend và OpenAPI bằng một typed contract duy nhất,
đồng thời giữ nguyên behavior nghiệp vụ đã được kiểm thử.

Phương án được chọn:

```text
Go typed API operations + Huma
              |
              +--> OpenAPI generated artifact
              +--> TypeScript client/types generated artifact
              +--> runtime contract tests
```

`net/http` vẫn là runtime HTTP chính. Huma chỉ là typed API boundary, không
được phép truy cập trực tiếp SQLite, SNMP hoặc repository.

## 2. Nguyên tắc bắt buộc

- Không migrate toàn bộ API trong một diff duy nhất.
- Không chạy song song hai production router cho cùng một route.
- Không xem `docs/openapi.yaml` hiện tại là đúng tuyệt đối; file này là
  baseline để phát hiện khác biệt, còn behavior đúng phải dựa trên nghiệp vụ,
  test và frontend usage.
- Không sửa thủ công generated OpenAPI hoặc generated TypeScript client.
- Không thay đổi business logic SNMP, counter, polling lease hoặc cartridge
  transaction chỉ vì migration framework.
- Mỗi nhóm endpoint phải có rollback rõ ràng trước khi chuyển production.
- Mọi thay đổi request/response phải có status code, error code và validation
  được kiểm thử.

## 3. Kiến trúc đích

```text
HTTP middleware
  - CORS
  - authentication
  - request limits
  - logging
        |
Huma typed operation
        |
Application service
        |
Repository / SNMP adapter / worker service
```

Đề xuất thư mục:

```text
backend/internal/api/
  operations/
    cartridge.go
    printer.go
    counter.go
    discovery.go
    jobs.go
  generated/
  errors.go
  server.go

backend/internal/application/
  cartridge_service.go
  printer_service.go
  counter_service.go

frontend/src/api/
  generated/
  appClient.ts
  queryKeys.ts
```

Generated files không được chỉnh tay.

## 4. Contract hiện tại cần kiểm kê

Trước khi chuyển, lập route matrix gồm:

| Nhóm | Route | Go handler | Frontend dùng | OpenAPI | Test runtime | Trạng thái |
| --- | --- | --- | --- | --- | --- | --- |
| Health | `/api/v1/health*` | Có | Có/không | cần rà | Có | `health` đã chuyển Huma; live/ready legacy |
| Profiles | `/api/v1/snmp/profiles` | Có | Có | Có | Có | Đã chuyển Huma |
| Discovery | `/api/v1/discovery/probe` | Có | Có | Có | Có | Chưa chuyển |
| Printers | `/api/v1/printers*` | Có | Có | Có | Có một phần | Collection đã chuyển Huma; detail/counter còn legacy |
| Counters | `/api/v1/printers/{id}/counters` | Có | Có | Có | Cần contract test | Chưa chuyển |
| Usage | `/api/v1/printers/{id}/usage` | Có | Có | Có | Cần contract test | Chưa chuyển |
| Poll/jobs | `/api/v1/printers/{id}/poll`, `/api/v1/jobs/{id}` | Có | Có | Có | Có | Chưa chuyển |
| Cartridge | `/api/v1/cartridges*` | Có | Có | Generated + baseline cần so sánh | Có | Đã chuyển runtime toàn bộ nhóm |

Route matrix phải được cập nhật sau mỗi nhóm migration.

## 5. Giai đoạn 0 — Chuẩn hóa contract

### Công việc

- Rà toàn bộ route thực tế từ `server.go`.
- Rà toàn bộ endpoint đang gọi trong `frontend/src/api/client.ts`.
- Rà schema/status/error trong `docs/openapi.yaml`.
- Xác định route thiếu hoặc response không thống nhất.
- Chuẩn hóa:
  - error envelope;
  - pagination;
  - RFC3339 timestamp;
  - UUID;
  - nullable field;
  - enum quality/status;
  - `operationId`.

### Kết quả cần có

- Route matrix đầy đủ.
- Danh sách breaking/non-breaking differences.
- Danh sách business behavior cần giữ nguyên.

## 6. Giai đoạn 1 — Pilot cartridge

Đã dựng typed contract tại `backend/internal/api/contract` và lệnh
`make api-contract`. Lệnh sinh `docs/openapi.huma.generated.yaml` để review
route/schema. Nhóm cartridge hiện đã chạy qua Huma runtime; các operation
giữ nguyên service/store và counter semantics, không dựng lại business logic.

Các checkpoint đã lưu:

- `7dc22e5`: shadow contract đầy đủ cho printer/counter/job.
- `b800d24`: health runtime.
- `2dbfe71`: profile catalog runtime.
- `f5ce494`: cartridge runtime migration hoàn tất.

Generated OpenAPI drift đã có gate `make api-contract-check` và CI sẽ fail nếu
artifact thay đổi mà chưa được commit.

Chuyển các operation:

```text
GET  /api/v1/cartridges
POST /api/v1/cartridges
POST /api/v1/cartridges/stock
POST /api/v1/cartridges/stock/refill-bottles
POST /api/v1/cartridges/replace
POST /api/v1/cartridges/refill
POST /api/v1/cartridges/refill-printer
GET  /api/v1/cartridges/logs
```

### Contract phải mô tả

- request/response typed;
- quantity phải lớn hơn 0;
- source type `new/refilled`;
- counter và `counter_quality`;
- lỗi thiếu tồn kho;
- lỗi SNMP được ghi `unavailable`, không đổi thành counter 0;
- authentication;
- transaction behavior.

### Kiểm thử bắt buộc

- request thiếu field;
- quantity bằng 0 hoặc âm;
- thay bình mới;
- bơm mực trực tiếp;
- thiếu bình trong kho;
- SNMP timeout;
- counter nhập thủ công;
- rollback khi ghi log thất bại;
- response thực tế validate được bằng contract.

Chỉ chuyển production sau khi nhóm cartridge đạt toàn bộ kiểm thử.

## 7. Giai đoạn 2 — Sinh OpenAPI và so sánh

Trong thời gian chuyển đổi, sinh file tạm:

```text
docs/openapi.huma.generated.yaml
```

So sánh theo cấu trúc, không diff text thuần:

- path/method;
- request schema;
- response schema;
- required fields;
- nullable;
- enum;
- status code;
- security;
- pagination;
- error schema.

Phân loại diff:

```text
MISSING       Huma thiếu endpoint đang chạy
ADDED         Huma có endpoint mới
SCHEMA_DIFF   request/response khác
STATUS_DIFF   HTTP status khác
SECURITY_DIFF auth khác
BEHAVIOR_DIFF runtime behavior khác
DOC_DIFF      chỉ khác mô tả/example
```

`BEHAVIOR_DIFF` phải được ưu tiên cao nhất. Không tự động sửa behavior chỉ để
file OpenAPI giống nhau.

## 8. Giai đoạn 3 — Chuyển router production

Sau khi pilot parity:

1. Giữ middleware chung hiện tại.
2. Đăng ký Huma router cho nhóm cartridge.
3. Xóa route thủ công tương ứng khỏi `http.ServeMux`.
4. Chạy integration test trên router production.
5. Giữ nguyên URL và status code đã cam kết.
6. Nếu có regression, rollback riêng nhóm cartridge.

Không được đăng ký cùng một method/path ở cả router cũ và Huma.

## 9. Giai đoạn 4 — Chuyển các nhóm còn lại

Thứ tự:

1. Printer registration/detail collection (collection đã chuyển một phần).
2. Counter readings và daily usage.
3. Polling jobs.
4. Discovery.
5. Hoàn tất health live/ready và error parity.

Lý do: cartridge có transaction rõ và dễ kiểm thử; discovery/profile có nhiều
credential và validation nên chuyển sau.

## 10. Frontend generated client

Sau khi contract của từng nhóm ổn định:

- Generate TypeScript types/client từ OpenAPI generated.
- Giữ wrapper nghiệp vụ trong `frontend/src/api/appClient.ts`.
- Xóa duplicate interface trong `frontend/src/api/types.ts` theo từng nhóm.
- Giữ TanStack Query và query keys ở frontend.
- Không đưa generated client trực tiếp vào component.

Frontend phải kiểm thử loading, error, empty và response status theo contract.

## 11. CI và deploy gate

CI bắt buộc chạy:

```bash
go generate ./...
go test ./...
go vet ./...
pnpm run api:generate
pnpm run typecheck
pnpm run format:check
pnpm run lint
pnpm run build
git diff --check
git diff --exit-code -- docs/openapi.yaml frontend/src/api/generated
```

Trước deploy:

1. Migrate database sạch.
2. Migrate database đã có dữ liệu mẫu.
3. Chạy API contract tests.
4. Kiểm tra endpoint health/readiness.
5. Kiểm tra frontend gọi đúng generated client.
6. Kiểm tra không log credential hoặc raw SNMP secret.
7. Chạy smoke test cartridge, counter và manual poll.

## 12. Tiêu chí hoàn thành migration

- Không còn DTO frontend trùng với generated type.
- Không còn route production trùng giữa Huma và mux cũ.
- `docs/openapi.yaml` được sinh tự động.
- Mọi route production có operation trong contract.
- Request/response thật validate được bằng schema.
- Error code/status ổn định.
- Go tests, vet, frontend checks và migration checks đạt.
- Có rollback plan cho từng nhóm endpoint.

## 13. Quyết định chưa được phép tự động hóa

Agent không được tự quyết định:

- đổi semantics counter;
- đổi status code đã dùng bởi frontend;
- đổi tên field public;
- nâng profile lên `verified`;
- thay đổi meaning của tồn kho `stock_new`, `stock_refilled`,
  `stock_refill_bottles`;
- xóa endpoint legacy nếu chưa có migration note.

Các thay đổi này cần review nghiệp vụ trước khi merge.

