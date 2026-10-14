# BÁO CÁO RÀ SOÁT FLEET-DUMP (HỆ THỐNG MÁY IN NPMS)

- **Tổng số file dump hợp lệ**: 20 máy in
- **Số nhóm dòng máy phân loại được**: 14 nhóm

## 1. Danh sách phân nhóm thiết bị và trạng thái Counter

| Nhóm | Hãng | Model | sysObjectID | Số lượng | Hỗ trợ Printer-MIB Marker | Không có Marker (Cần Vendor Fallback) |
|---|---|---|---|:---:|:---:|:---:|
| 1 | HP | HP LaserJet Pro M501dn | `1.3.6.1.4.1.11.2.3.9.1` | 3 máy | 3 | **0** |
| 2 | HP | HP LaserJet M402dn | `1.3.6.1.4.1.11.2.3.9.1` | 3 máy | 3 | **0** |
| 3 | Ricoh | IM 2500 | `1.3.6.1.4.1.367.1.1` | 2 máy | 2 | **0** |
| 4 | Brother | Brother NC-04h, Firmware Ver.U  ,MID 8WD-501 | `1.3.6.1.4.1.2435.2.3.9.1` | 2 máy | 0 | **2** |
| 5 | Brother | BRNB42200B39DB8 | `1.3.6.1.4.1.2435.2.3.9.1` | 1 máy | 1 | **0** |
| 6 | HP | HP87DAF4 | `1.3.6.1.4.1.11.2.3.9.1` | 1 máy | 1 | **0** |
| 7 | Brother | BRN94DDF8E76184 | `1.3.6.1.4.1.2435.2.3.9.1` | 1 máy | 1 | **0** |
| 8 | Brother | BRNB42200B3C509 | `1.3.6.1.4.1.2435.2.3.9.1` | 1 máy | 1 | **0** |
| 9 | Khác | ET788C77493D74 | `1.3.6.1.4.1.641.51.71107404` | 1 máy | 1 | **0** |
| 10 | Brother | BRN94DDF8E43CFA | `1.3.6.1.4.1.2435.2.3.9.1` | 1 máy | 1 | **0** |
| 11 | Brother | Brother NC-9200h, Firmware Ver.ZL  ,MID 84E-821 | `1.3.6.1.4.1.2435.2.3.9.1` | 1 máy | 1 | **0** |
| 12 | Brother | BRN94DDF8E43CD4 | `1.3.6.1.4.1.2435.2.3.9.1` | 1 máy | 1 | **0** |
| 13 | Brother | Brother NC-10h, Firmware Ver.2.28, MID 5WD-B03 | `1.3.6.1.4.1.2435.2.3.9.1` | 1 máy | 0 | **1** |
| 14 | Brother | BRNB42200B39DCE | `1.3.6.1.4.1.2435.2.3.9.1` | 1 máy | 1 | **0** |

## 2. Chi tiết từng dòng máy và OID Counter phát hiện được

### Nhóm 1: HP - HP LaserJet Pro M501dn (3 máy)
- **sysObjectID**: `1.3.6.1.4.1.11.2.3.9.1`
- **Danh sách IP**: 10.141.6.5, 10.141.6.8, 10.141.6.9

**Marker Table (RFC 3805 - PrtMarkerLifeCount):**
- Instance `1.1`: Life = **0** (impressions, unit code 0)

**Top Candidate OIDs đếm trang (Page Counters):**
| Tên / Khóa | OID | Giá trị hiện tại | Đơn vị | Nguồn phát hiện |
|---|---|:---:|:---:|---|
| `marker_life.1.1` | `1.3.6.1.2.1.43.10.2.1.4.1.1` | **398859** | impressions | Printer-MIB (prtMarkerLifeCount) |
| `vendor_walk.1.3.6.1.4.1.11.2.3.9.4.2.1.4.1.2.5.0` | `1.3.6.1.4.1.11.2.3.9.4.2.1.4.1.2.5.0` | **398859** | pages | Vendor Subtree (11) |
| `vendor_walk.1.3.6.1.4.1.11.2.3.9.4.2.1.4.1.2.6.0` | `1.3.6.1.4.1.11.2.3.9.4.2.1.4.1.2.6.0` | **398574** | pages | Vendor Subtree (11) |
| `vendor_walk.1.3.6.1.4.1.11.2.3.9.4.2.1.4.1.2.22.0` | `1.3.6.1.4.1.11.2.3.9.4.2.1.4.1.2.22.0` | **164846** | pages | Vendor Subtree (11) |
| `vendor_walk.1.3.6.1.4.1.11.2.3.9.4.2.1.4.1.2.34.0` | `1.3.6.1.4.1.11.2.3.9.4.2.1.4.1.2.34.0` | **366** | pages | Vendor Subtree (11) |
| `vendor_walk.1.3.6.1.4.1.11.2.3.9.4.2.1.4.1.2.35.0` | `1.3.6.1.4.1.11.2.3.9.4.2.1.4.1.2.35.0` | **1** | pages | Vendor Subtree (11) |

---

### Nhóm 2: HP - HP LaserJet M402dn (3 máy)
- **sysObjectID**: `1.3.6.1.4.1.11.2.3.9.1`
- **Danh sách IP**: 10.141.6.19, 10.141.6.3, 10.141.6.44

**Marker Table (RFC 3805 - PrtMarkerLifeCount):**
- Instance `1.1`: Life = **0** (impressions, unit code 0)

**Top Candidate OIDs đếm trang (Page Counters):**
| Tên / Khóa | OID | Giá trị hiện tại | Đơn vị | Nguồn phát hiện |
|---|---|:---:|:---:|---|
| `marker_life.1.1` | `1.3.6.1.2.1.43.10.2.1.4.1.1` | **1652** | impressions | Printer-MIB (prtMarkerLifeCount) |
| `vendor_walk.1.3.6.1.4.1.11.2.3.9.4.2.1.4.1.2.5.0` | `1.3.6.1.4.1.11.2.3.9.4.2.1.4.1.2.5.0` | **1652** | pages | Vendor Subtree (11) |
| `vendor_walk.1.3.6.1.4.1.11.2.3.9.4.2.1.4.1.2.6.0` | `1.3.6.1.4.1.11.2.3.9.4.2.1.4.1.2.6.0` | **1652** | pages | Vendor Subtree (11) |
| `vendor_walk.1.3.6.1.4.1.11.2.3.9.4.2.1.4.1.2.22.0` | `1.3.6.1.4.1.11.2.3.9.4.2.1.4.1.2.22.0` | **673** | pages | Vendor Subtree (11) |
| `vendor_walk.1.3.6.1.4.1.11.2.3.9.4.2.1.4.1.2.34.0` | `1.3.6.1.4.1.11.2.3.9.4.2.1.4.1.2.34.0` | **5** | pages | Vendor Subtree (11) |
| `vendor_walk.1.3.6.1.4.1.11.2.3.9.4.2.1.4.1.2.49.0` | `1.3.6.1.4.1.11.2.3.9.4.2.1.4.1.2.49.0` | **1** | pages | Vendor Subtree (11) |

---

### Nhóm 3: Ricoh - IM 2500 (2 máy)
- **sysObjectID**: `1.3.6.1.4.1.367.1.1`
- **Danh sách IP**: 10.141.6.10, 10.141.6.12

**Marker Table (RFC 3805 - PrtMarkerLifeCount):**
- Instance `1.1`: Life = **0** (sheets, unit code 0)

**Top Candidate OIDs đếm trang (Page Counters):**
| Tên / Khóa | OID | Giá trị hiện tại | Đơn vị | Nguồn phát hiện |
|---|---|:---:|:---:|---|
| `vendor_walk.1.3.6.1.4.1.367.3.2.1.2.19.5.1.9.104` | `1.3.6.1.4.1.367.3.2.1.2.19.5.1.9.104` | **2302550** | pages | Vendor Subtree (367) |
| `vendor_walk.1.3.6.1.4.1.367.3.2.1.2.19.5.1.9.92` | `1.3.6.1.4.1.367.3.2.1.2.19.5.1.9.92` | **2302550** | pages | Vendor Subtree (367) |
| `vendor_walk.1.3.6.1.4.1.367.3.2.1.2.19.5.1.9.151` | `1.3.6.1.4.1.367.3.2.1.2.19.5.1.9.151` | **1528869** | pages | Vendor Subtree (367) |
| `vendor_walk.1.3.6.1.4.1.367.3.2.1.2.19.5.1.9.146` | `1.3.6.1.4.1.367.3.2.1.2.19.5.1.9.146` | **1298374** | pages | Vendor Subtree (367) |
| `vendor_walk.1.3.6.1.4.1.367.3.2.1.2.19.5.1.9.102` | `1.3.6.1.4.1.367.3.2.1.2.19.5.1.9.102` | **1107323** | pages | Vendor Subtree (367) |
| `vendor_walk.1.3.6.1.4.1.367.3.2.1.2.19.5.1.9.91` | `1.3.6.1.4.1.367.3.2.1.2.19.5.1.9.91` | **1107323** | pages | Vendor Subtree (367) |

---

### Nhóm 4: Brother - Brother NC-04h, Firmware Ver.U  ,MID 8WD-501 (2 máy)
- **sysObjectID**: `1.3.6.1.4.1.2435.2.3.9.1`
- **Danh sách IP**: 10.141.6.40, 10.141.6.41
- ⚠️ **Các máy KHÔNG trả về Printer-MIB Marker**: 10.141.6.40, 10.141.6.41

**Marker Table (RFC 3805 - PrtMarkerLifeCount):**
- *Không có marker row nào được trả về.*

**Top Candidate OIDs đếm trang (Page Counters):**
| Tên / Khóa | OID | Giá trị hiện tại | Đơn vị | Nguồn phát hiện |
|---|---|:---:|:---:|---|
| `vendor_walk.1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.54.2.2.1.3.2` | `1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.54.2.2.1.3.2` | **400695** | pages | Vendor Subtree (2435) |
| `vendor_walk.1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.54.2.2.1.3.3` | `1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.54.2.2.1.3.3` | **400695** | pages | Vendor Subtree (2435) |
| `vendor_walk.1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.6.0` | `1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.6.0` | **71001** | pages | Vendor Subtree (2435) |
| `vendor_walk.1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.4.0` | `1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.4.0` | **204** | pages | Vendor Subtree (2435) |
| `vendor_walk.1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.51.2.1.1.10` | `1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.51.2.1.1.10` | **10** | pages | Vendor Subtree (2435) |
| `vendor_walk.1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.51.1.0` | `1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.51.1.0` | **10** | pages | Vendor Subtree (2435) |

---

### Nhóm 5: Brother - BRNB42200B39DB8 (1 máy)
- **sysObjectID**: `1.3.6.1.4.1.2435.2.3.9.1`
- **Danh sách IP**: 10.141.6.13

**Marker Table (RFC 3805 - PrtMarkerLifeCount):**
- Instance `1.1`: Life = **0** (sheets, unit code 0)

**Top Candidate OIDs đếm trang (Page Counters):**
| Tên / Khóa | OID | Giá trị hiện tại | Đơn vị | Nguồn phát hiện |
|---|---|:---:|:---:|---|
| `marker_life.1.1` | `1.3.6.1.2.1.43.10.2.1.4.1.1` | **151142** | sheets | Printer-MIB (prtMarkerLifeCount) |
| `vendor_walk.1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.52.31.1.3.1` | `1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.52.31.1.3.1` | **151142** | pages | Vendor Subtree (2435) |
| `vendor_walk.1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.52.2.1.3.2` | `1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.52.2.1.3.2` | **150960** | pages | Vendor Subtree (2435) |
| `vendor_walk.1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.52.1.1.3.1` | `1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.52.1.1.3.1` | **150960** | pages | Vendor Subtree (2435) |
| `vendor_walk.1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.52.21.1.3.1` | `1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.52.21.1.3.1` | **150958** | pages | Vendor Subtree (2435) |
| `vendor_walk.1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.51.2.1.3.1` | `1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.51.2.1.3.1` | **149220** | pages | Vendor Subtree (2435) |

---

### Nhóm 6: HP - HP87DAF4 (1 máy)
- **sysObjectID**: `1.3.6.1.4.1.11.2.3.9.1`
- **Danh sách IP**: 10.141.6.58

**Marker Table (RFC 3805 - PrtMarkerLifeCount):**
- Instance `1.1`: Life = **0** (impressions, unit code 0)

**Top Candidate OIDs đếm trang (Page Counters):**
| Tên / Khóa | OID | Giá trị hiện tại | Đơn vị | Nguồn phát hiện |
|---|---|:---:|:---:|---|
| `marker_life.1.1` | `1.3.6.1.2.1.43.10.2.1.4.1.1` | **27797** | impressions | Printer-MIB (prtMarkerLifeCount) |
| `hp_total_page_count` | `1.3.6.1.4.1.11.2.3.9.4.2.1.4.1.2.6.0` | **27797** | pages | HP-Vendor |
| `hp_engine_cycles` | `1.3.6.1.4.1.11.2.3.9.4.2.1.4.1.2.5.0` | **27797** | impressions | HP-Vendor |
| `vendor_walk.1.3.6.1.4.1.11.2.3.9.4.2.1.4.1.2.22.0` | `1.3.6.1.4.1.11.2.3.9.4.2.1.4.1.2.22.0` | **6075** | pages | Vendor Subtree (11) |

---

### Nhóm 7: Brother - BRN94DDF8E76184 (1 máy)
- **sysObjectID**: `1.3.6.1.4.1.2435.2.3.9.1`
- **Danh sách IP**: 10.141.6.50

**Marker Table (RFC 3805 - PrtMarkerLifeCount):**
- Instance `1.1`: Life = **0** (sheets, unit code 0)

**Top Candidate OIDs đếm trang (Page Counters):**
| Tên / Khóa | OID | Giá trị hiện tại | Đơn vị | Nguồn phát hiện |
|---|---|:---:|:---:|---|
| `vendor_walk.1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.6.0` | `1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.6.0` | **40000** | pages | Vendor Subtree (2435) |
| `marker_life.1.1` | `1.3.6.1.2.1.43.10.2.1.4.1.1` | **16623** | sheets | Printer-MIB (prtMarkerLifeCount) |
| `vendor_walk.1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.52.31.1.3.1` | `1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.52.31.1.3.1` | **16623** | pages | Vendor Subtree (2435) |
| `vendor_walk.1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.52.21.1.3.1` | `1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.52.21.1.3.1` | **16623** | pages | Vendor Subtree (2435) |
| `vendor_walk.1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.52.1.1.3.1` | `1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.52.1.1.3.1` | **16614** | pages | Vendor Subtree (2435) |
| `vendor_walk.1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.52.2.1.3.2` | `1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.52.2.1.3.2` | **16610** | pages | Vendor Subtree (2435) |

---

### Nhóm 8: Brother - BRNB42200B3C509 (1 máy)
- **sysObjectID**: `1.3.6.1.4.1.2435.2.3.9.1`
- **Danh sách IP**: 10.141.6.16

**Marker Table (RFC 3805 - PrtMarkerLifeCount):**
- Instance `1.1`: Life = **0** (sheets, unit code 0)

**Top Candidate OIDs đếm trang (Page Counters):**
| Tên / Khóa | OID | Giá trị hiện tại | Đơn vị | Nguồn phát hiện |
|---|---|:---:|:---:|---|
| `marker_life.1.1` | `1.3.6.1.2.1.43.10.2.1.4.1.1` | **163923** | sheets | Printer-MIB (prtMarkerLifeCount) |
| `vendor_walk.1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.52.31.1.3.1` | `1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.52.31.1.3.1` | **163918** | pages | Vendor Subtree (2435) |
| `vendor_walk.1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.52.2.1.3.2` | `1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.52.2.1.3.2` | **163767** | pages | Vendor Subtree (2435) |
| `vendor_walk.1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.52.1.1.3.1` | `1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.52.1.1.3.1` | **163767** | pages | Vendor Subtree (2435) |
| `vendor_walk.1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.52.21.1.3.1` | `1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.52.21.1.3.1` | **163731** | pages | Vendor Subtree (2435) |
| `vendor_walk.1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.51.2.1.3.1` | `1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.51.2.1.3.1` | **161495** | pages | Vendor Subtree (2435) |

---

### Nhóm 9: Không xác định - ET788C77493D74 (1 máy)
- **sysObjectID**: `1.3.6.1.4.1.641.51.71107404`
- **Danh sách IP**: 10.141.6.49

**Marker Table (RFC 3805 - PrtMarkerLifeCount):**
- Instance `1.1`: Life = **0** (impressions, unit code 0)

**Top Candidate OIDs đếm trang (Page Counters):**
| Tên / Khóa | OID | Giá trị hiện tại | Đơn vị | Nguồn phát hiện |
|---|---|:---:|:---:|---|
| `marker_life.1.1` | `1.3.6.1.2.1.43.10.2.1.4.1.1` | **2201** | impressions | Printer-MIB (prtMarkerLifeCount) |

---

### Nhóm 10: Brother - BRN94DDF8E43CFA (1 máy)
- **sysObjectID**: `1.3.6.1.4.1.2435.2.3.9.1`
- **Danh sách IP**: 10.141.6.31

**Marker Table (RFC 3805 - PrtMarkerLifeCount):**
- Instance `1.1`: Life = **0** (sheets, unit code 0)

**Top Candidate OIDs đếm trang (Page Counters):**
| Tên / Khóa | OID | Giá trị hiện tại | Đơn vị | Nguồn phát hiện |
|---|---|:---:|:---:|---|
| `marker_life.1.1` | `1.3.6.1.2.1.43.10.2.1.4.1.1` | **15415** | sheets | Printer-MIB (prtMarkerLifeCount) |
| `vendor_walk.1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.52.31.1.3.1` | `1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.52.31.1.3.1` | **15415** | pages | Vendor Subtree (2435) |
| `vendor_walk.1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.52.1.1.3.1` | `1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.52.1.1.3.1` | **15413** | pages | Vendor Subtree (2435) |
| `vendor_walk.1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.52.21.1.3.1` | `1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.52.21.1.3.1` | **15410** | pages | Vendor Subtree (2435) |
| `vendor_walk.1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.52.2.1.3.2` | `1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.52.2.1.3.2` | **15408** | pages | Vendor Subtree (2435) |
| `vendor_walk.1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.51.2.1.3.1` | `1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.51.2.1.3.1` | **15317** | pages | Vendor Subtree (2435) |

---

### Nhóm 11: Brother - Brother NC-9200h, Firmware Ver.ZL  ,MID 84E-821 (1 máy)
- **sysObjectID**: `1.3.6.1.4.1.2435.2.3.9.1`
- **Danh sách IP**: 10.141.6.37

**Marker Table (RFC 3805 - PrtMarkerLifeCount):**
- Instance `1.1`: Life = **0** (sheets, unit code 0)

**Top Candidate OIDs đếm trang (Page Counters):**
| Tên / Khóa | OID | Giá trị hiện tại | Đơn vị | Nguồn phát hiện |
|---|---|:---:|:---:|---|
| `vendor_walk.1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.6.0` | `1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.6.0` | **40000** | pages | Vendor Subtree (2435) |
| `marker_life.1.1` | `1.3.6.1.2.1.43.10.2.1.4.1.1` | **5481** | sheets | Printer-MIB (prtMarkerLifeCount) |
| `vendor_walk.1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.51.2.1.3.1` | `1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.51.2.1.3.1` | **5462** | pages | Vendor Subtree (2435) |
| `vendor_walk.1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.52.21.1.3.1` | `1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.52.21.1.3.1` | **5441** | pages | Vendor Subtree (2435) |
| `vendor_walk.1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.51.2.1.3.2` | `1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.51.2.1.3.2` | **5425** | pages | Vendor Subtree (2435) |
| `vendor_walk.1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.51.2.1.3.3` | `1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.51.2.1.3.3` | **5138** | pages | Vendor Subtree (2435) |

---

### Nhóm 12: Brother - BRN94DDF8E43CD4 (1 máy)
- **sysObjectID**: `1.3.6.1.4.1.2435.2.3.9.1`
- **Danh sách IP**: 10.141.6.45

**Marker Table (RFC 3805 - PrtMarkerLifeCount):**
- Instance `1.1`: Life = **0** (sheets, unit code 0)

**Top Candidate OIDs đếm trang (Page Counters):**
| Tên / Khóa | OID | Giá trị hiện tại | Đơn vị | Nguồn phát hiện |
|---|---|:---:|:---:|---|
| `vendor_walk.1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.6.0` | `1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.6.0` | **40000** | pages | Vendor Subtree (2435) |
| `marker_life.1.1` | `1.3.6.1.2.1.43.10.2.1.4.1.1` | **5908** | sheets | Printer-MIB (prtMarkerLifeCount) |
| `vendor_walk.1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.52.1.1.3.1` | `1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.52.1.1.3.1` | **5908** | pages | Vendor Subtree (2435) |
| `vendor_walk.1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.52.31.1.3.1` | `1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.52.31.1.3.1` | **5908** | pages | Vendor Subtree (2435) |
| `vendor_walk.1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.52.21.1.3.1` | `1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.52.21.1.3.1` | **5898** | pages | Vendor Subtree (2435) |
| `vendor_walk.1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.52.2.1.3.2` | `1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.52.2.1.3.2` | **5896** | pages | Vendor Subtree (2435) |

---

### Nhóm 13: Brother - Brother NC-10h, Firmware Ver.2.28, MID 5WD-B03 (1 máy)
- **sysObjectID**: `1.3.6.1.4.1.2435.2.3.9.1`
- **Danh sách IP**: 10.141.6.42
- ⚠️ **Các máy KHÔNG trả về Printer-MIB Marker**: 10.141.6.42

**Marker Table (RFC 3805 - PrtMarkerLifeCount):**
- *Không có marker row nào được trả về.*

**Top Candidate OIDs đếm trang (Page Counters):**
| Tên / Khóa | OID | Giá trị hiện tại | Đơn vị | Nguồn phát hiện |
|---|---|:---:|:---:|---|
| `vendor_walk.1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.54.2.2.1.3.3` | `1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.54.2.2.1.3.3` | **59845** | pages | Vendor Subtree (2435) |
| `vendor_walk.1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.54.2.2.1.3.2` | `1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.54.2.2.1.3.2` | **59806** | pages | Vendor Subtree (2435) |
| `vendor_walk.1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.6.0` | `1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.6.0` | **10001** | pages | Vendor Subtree (2435) |
| `vendor_walk.1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.54.2.2.1.3.1` | `1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.54.2.2.1.3.1` | **39** | pages | Vendor Subtree (2435) |
| `vendor_walk.1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.54.2.2.1.2.3` | `1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.54.2.2.1.2.3` | **4** | pages | Vendor Subtree (2435) |
| `vendor_walk.1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.59.2.1.2.2` | `1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.59.2.1.2.2` | **3** | pages | Vendor Subtree (2435) |

---

### Nhóm 14: Brother - BRNB42200B39DCE (1 máy)
- **sysObjectID**: `1.3.6.1.4.1.2435.2.3.9.1`
- **Danh sách IP**: 10.141.6.14

**Marker Table (RFC 3805 - PrtMarkerLifeCount):**
- Instance `1.1`: Life = **0** (sheets, unit code 0)

**Top Candidate OIDs đếm trang (Page Counters):**
| Tên / Khóa | OID | Giá trị hiện tại | Đơn vị | Nguồn phát hiện |
|---|---|:---:|:---:|---|
| `vendor_walk.1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.52.31.1.3.1` | `1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.52.31.1.3.1` | **148979** | pages | Vendor Subtree (2435) |
| `marker_life.1.1` | `1.3.6.1.2.1.43.10.2.1.4.1.1` | **148979** | sheets | Printer-MIB (prtMarkerLifeCount) |
| `vendor_walk.1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.52.1.1.3.1` | `1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.52.1.1.3.1` | **148826** | pages | Vendor Subtree (2435) |
| `vendor_walk.1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.52.2.1.3.2` | `1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.52.2.1.3.2` | **148826** | pages | Vendor Subtree (2435) |
| `vendor_walk.1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.52.21.1.3.1` | `1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.52.21.1.3.1` | **148794** | pages | Vendor Subtree (2435) |
| `vendor_walk.1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.51.2.1.3.1` | `1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.51.2.1.3.1` | **147034** | pages | Vendor Subtree (2435) |

---

