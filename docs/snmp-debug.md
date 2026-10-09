# Hướng dẫn sử dụng CLI `snmp-debug` (SNMP Diagnostic Tool)

`snmp-debug` là công cụ dòng lệnh (CLI) chẩn đoán SNMP độc lập trong NPMS. Công cụ hỗ trợ giao thức SNMP v2c và v3 (bao gồm xác thực `authPriv`), giúp kiểm tra máy in trong mạng LAN, đọc các giá trị OID/counter và xuất dữ liệu JSON mẫu mà không lưu credential hay dữ liệu vào cơ sở dữ liệu SQLite.

---

## 1. Phương thức chạy (How to Run)

### 1.1 Chạy từ Binary đã build (thư mục `dist/` hoặc `bin/`)

* **Windows PowerShell**:
  ```powershell
  .\dist\bin\npms-snmp-debug.exe <command> [options]
  # Hoặc nếu chạy từ thư mục dist:
  .\bin\npms-snmp-debug.exe <command> [options]
  ```

* **Linux / macOS**:
  ```bash
  ./dist/bin/npms-snmp-debug <command> [options]
  ```

### 1.2 Chạy trực tiếp từ Go Source (Môi trường phát triển)

```bash
cd backend
go run ./cmd/snmp-debug <command> [options]
```

---

## 2. Danh sách Lệnh (Subcommands)

| Lệnh (`command`) | Mô tả | Cờ yêu cầu |
| :--- | :--- | :--- |
| `check` | Hiển thị bảng thông tin máy in và counter dạng console dễ đọc. | `--host` |
| `probe` | Đọc các OID định danh hệ thống (`sysDescr`, `sysObjectID`, `sysName`, `serial`) và bảng Marker Life/Unit dưới dạng JSON. | `--host` |
| `get` | Đọc giá trị của duy nhất 1 OID cụ thể. | `--host`, `--oid` |
| `walk` | Duyệt (Walk) toàn bộ cây OID bắt đầu từ OID chỉ định. | `--host`, `--oid` |
| `export` | Quét toàn bộ thông tin máy in và xuất ra file JSON. | `--host`, `--output` |

---

## 3. Danh sách các Cờ Tùy chọn (Flags & Parameters)

### Cấu hình kết nối SNMP

* `--host` *(Chuỗi, bắt buộc)*: Địa chỉ IP hoặc Hostname của máy in. (Ví dụ: `192.168.1.50`).
* `--port` *(Số nguyên, mặc định: `161`)*: Cổng UDP giao tiếp SNMP.
* `--version` *(Chuỗi, mặc định: `2c`)*: Phiên bản SNMP (`2c` hoặc `3`).
* `--timeout` *(Số nguyên, mặc định: `3`)*: Thời gian chờ phản hồi (tính bằng giây).
* `--retries` *(Số nguyên, mặc định: `1`)*: Số lần thử lại nếu truy vấn thất bại.

### SNMP v2c Authentication
* `--community` *(Chuỗi)*: Chuỗi Community string cho SNMP v2c (ví dụ: `public`, `private`).

### SNMP v3 Authentication & Security
* `--username` *(Chuỗi)*: Tên người dùng SNMPv3.
* `--auth-protocol` *(Chuỗi)*: Giao thức xác thực SNMPv3 (`MD5` hoặc `SHA`).
* `--auth-passphrase` *(Chuỗi)*: Mật khẩu xác thực (Auth Passphrase).
* `--priv-protocol` *(Chuỗi)*: Giao thức mã hóa riêng tư SNMPv3 (`DES` hoặc `AES`).
* `--priv-passphrase` *(Chuỗi)*: Mật khẩu mã hóa (Privacy Passphrase).

### Tham số Truy vấn & Xuất Dữ liệu
* `--oid` *(Chuỗi)*: Mã OID cần truy vấn cho lệnh `get` hoặc `walk` (ví dụ: `1.3.6.1.2.1.43.10.2.1.4`).
* `--max-repetitions` *(Số nguyên 1-255, mặc định: `25`)*: Số dòng trả về tối đa trong mỗi gói tin Bulk Walk.
* `--output` *(Chuỗi)*: Đường dẫn lưu file kết quả JSON cho lệnh `export` (ví dụ: `sample.json`).

---

## 4. Các ví dụ câu lệnh thực tế (Usage Examples)

### 4.1 Kiểm tra nhanh trạng thái máy in (`check`)

Kiểm tra thông tin máy in HP/Brother/Ricoh qua SNMP v2c:
```powershell
.\dist\bin\npms-snmp-debug.exe check --host 192.168.1.100 --version 2c --community public
```

**Kết quả mẫu hiển thị trên Console:**
```text
NPMS SNMP check
Host/IP: 192.168.1.100
Status:  OK

Printer information:
  Name:        HP-M501DN-OFFICE
  Description: HP ETHERNET MULTI-FUNCTION PRINTER
  Object ID:   1.3.6.1.4.1.11.2.3.9.1
  Serial:      VNB3K12345

Counter values:
  INSTANCE     UNIT    LIFE_COUNT
  1.1          1       124500
```

---

### 4.2 Lấy thông tin máy in bằng SNMP v3 (`authPriv`)

```powershell
.\dist\bin\npms-snmp-debug.exe check --host 192.168.1.105 `
  --version 3 `
  --username admin `
  --auth-protocol SHA --auth-passphrase "AuthPass123" `
  --priv-protocol AES --priv-passphrase "PrivPass123"
```

---

### 4.3 Đọc một OID cụ thể (`get`)

Ví dụ đọc OID `sysDescr.0` (`1.3.6.1.2.1.1.1.0`):
```powershell
.\dist\bin\npms-snmp-debug.exe get --host 192.168.1.100 --community public --oid 1.3.6.1.2.1.1.1.0
```

---

### 4.4 Duyệt cây OID (`walk`)

Ví dụ duyệt bảng vật tư máy in Printer-MIB Subtree (`1.3.6.1.2.1.43.11.1.1`):
```powershell
.\dist\bin\npms-snmp-debug.exe walk --host 192.168.1.100 --community public --oid 1.3.6.1.2.1.43.11.1.1
```

---

### 4.5 Xuất dữ liệu JSON để mẫu làm Profile (`export`)

Xuất kết quả chẩn đoán ra file JSON mẫu phục vụ phân tích OID và xây dựng Profile thiết bị:
```powershell
.\dist\bin\npms-snmp-debug.exe export --host 192.168.1.100 --community public --output ./sample-hp-m501.json
```

---

## 5. Lưu ý an toàn & Bảo mật (Security & Safety)

1. **Không ghi credential**: CLI `snmp-debug` không bao giờ hiển thị hoặc lưu lại Community string hay SNMPv3 Passphrase trong màn hình kết quả hoặc file JSON xuất ra.
2. **Không ghi đè Database**: Lệnh `snmp-debug` hoạt động ở chế độ Read-Only, không ghi bất kỳ dữ liệu nào vào cơ sở dữ liệu SQLite `npms.db`.
3. **Phân biệt Counter**: Giá trị `LIFE_COUNT` thu được từ SNMP Marker chỉ được ghép khi trùng khớp `INSTANCE`. Không tự ý quy đổi marker value thành số trang in vật lý nếu chưa xác minh với trang Configuration Page hoặc màn hình điều khiển máy in.
