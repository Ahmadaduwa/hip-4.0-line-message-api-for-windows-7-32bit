# HIP 4.0 Notify (LINE & Telegram) for Windows 7 32-bit

[![CI Build & Test](https://github.com/Ahmadaduwa/hip-4.0-notify/actions/workflows/ci.yml/badge.svg)](https://github.com/Ahmadaduwa/hip-4.0-notify/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/badge/Go-1.20-00ADD8?logo=go)](https://go.dev/)
[![Platform](https://img.shields.io/badge/Platform-Windows%207%20(32--bit%20x86)-blue?logo=windows)](https://microsoft.com)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

ระบบสรุปเวลาสแกนนิ้ว/ใบหน้าและส่งรายงานการมาทำงานเข้า **LINE Messaging API (LINE Official Account)** และ **Telegram Bot** พร้อมระบบแจ้งเตือนสแกนนิ้ว **Realtime** อัตโนมัติ เชื่อมต่อกับฐานข้อมูลของโปรแกรม **HIP Premium Time / HIP TIME 4.0 (SQL Server)** ออกแบบและปรับแต่งเป็นพิเศษให้ทำงานได้อย่างเสถียรบนระบบปฏิบัติการรุ่นเก่าอย่าง **Windows 7 32-bit (x86)** โดยเฉพาะ

---

## 🌟 จุดเด่นและคุณสมบัติ (Key Features)

- **รองรับ Windows 7 (32-bit x86) 100%:** คอมไพล์ด้วย Go 1.20 (`GOOS=windows GOARCH=386`) ทำงานแบบ Standalone ไม่ต้องลง Runtime หรือ Library เพิ่มเติม
- **รายงานสรุปเรียงตามเวลาสแกนจริง:**
  - ภายในกลุ่ม **"มาตรงเวลา"** และ **"มาสาย"** เรียงลำดับตามเวลาสแกนแรกจริง (`FirstScan`) จากเช้าไปสายอย่างแม่นยำ
  - จัดหมวดหมู่ชัดเจน: **❌ ขาด $\rightarrow$ 📝 ลา $\rightarrow$ ⏰ สาย $\rightarrow$ ✅ ปกติ**
  - **LINE จำกัดโควตาปลอดภัย (~100 คน) / Telegram แสดงรายชื่อครบ 100% ทุกคน:** ฝั่ง LINE จัดสรรโควตาไม่ให้เกินขีดจำกัด 5,000 ตัวอักษรและโควตาข้อความรายเดือน ส่วนฝั่ง Telegram แสดงรายชื่อครบทุกคนโดยไม่จำกัด (และแบ่งส่งหลายข้อความอัตโนมัติหากเกิน 4,000 ตัวอักษร)
- **Telegram Bot แจ้งเตือนสแกนนิ้วแบบ Realtime:**
  - ตรวจจับข้อมูลสแกนใหม่จาก SQL Server (`Transcantime`) ทุกๆ 10 วินาที
  - แจ้งเตือนชื่อครูพร้อมเวลาสแกนจริงถึงระดับวินาทีจาก DB: `✅ [ชื่อ] — 07:12:34 น. (ตรงเวลา)` หรือ `⏰ [ชื่อ] — 07:45:10 น. (สาย 15 นาที)` และสแกนรอบถัดไป `🔄 [ชื่อ] — 16:30:05 น. (สแกนครั้งที่ N)` (หากไม่พบชื่อใน DB จะแสดงเป็น `รหัส <ID>`)
  - ระบบ Smart Deduplication: ตัดสแกนซ้ำของคนเดิมภายใน 2 นาทีออกอัตโนมัติ
  - กรณีสแกนพร้อมกันเกิน 5 รายชื่อ ระบบจะรวมเป็น 1 ข้อความสรุปเรียงตามเวลาสแกน
  - บันทึกประวัติที่ส่งแล้วลง `data/telegram_sent.json` เมื่อเปิดโปรแกรมใหม่จะไม่ส่งซ้ำ
- **ระบบ Dynamic Summary (ส่งรายงานสรุปอัจฉริยะ ทั้ง LINE และ Telegram):**
  - ใช้ตารางเวลาส่งสรุปร่วมกันทั้ง LINE และ Telegram (ตั้งค่าได้หลายรอบต่อวัน เช่น `09:00`, `18:00`)
  - ก่อนส่งในแต่ละรอบ ระบบจะตรวจสอบว่า **มีการเช็คชื่อเพิ่มขึ้นจากรอบส่งสรุปครั้งก่อนหน้าของวันนั้นหรือไม่**
  - หากไม่มีการเช็คชื่อเพิ่ม (หรือไม่มีใครสแกนเลยในวันหยุดนักขัตฤกษ์/เสาร์-อาทิตย์) ระบบจะไม่ส่งรายงานซ้ำซ้อนโดยอัตโนมัติ
  - สวิตช์แยกอิสระต่อผู้รับบน Telegram: เลือกว่าจะรับเฉพาะ **Realtime**, **สรุป**, หรือทั้งสองอย่าง
- **Web Dashboard จัดการระบบ (Port 8090):**
  - กำหนดรอบเวลาส่งสรุปของ LINE และ Telegram ได้ไม่จำกัดรอบต่อวันผ่าน UI แบบรายการ (+ เพิ่มรอบ / ลบ)
  - จัดการและตั้งค่าการเชื่อมต่อทั้ง LINE และ Telegram Bot ผ่านหน้าเว็บได้ 100%
  - ระบบ **Auto-Add:** เมื่อคุณครูหรือกลุ่มแอด LINE หรือพิมพ์ `/start` เข้า Telegram Bot ระบบจะลงทะเบียนผู้รับให้อัตโนมัติ (ใช้ Long Polling ไม่ต้องเปิดพอร์ตภายนอก)
  - ตรวจสอบสถานะการเชื่อมต่อ SQL Server และดูพรีวิวข้อความแบบ Live
- **สร้างไฟล์ Excel สรุปประจำวันอัตโนมัติ (Daily Excel Report):**
  - ส่งออกไฟล์ `reports/attendance-YYYY-MM-DD.xlsx` ลงเครื่องอัตโนมัติทุกครั้งที่รายงานเข้า LINE และ Telegram
  - แสดงเกณฑ์เวลาเข้างานที่ชัดเจน พร้อมตารางข้อมูล `ชื่อ`, `สถานะ`, `เวลาที่เข้า`
  - ลบไฟล์ Excel และไฟล์ Log เก่าย้อนหลังเกิน 30 วันอัตโนมัติ (Daily Retention)
- **ระบบความปลอดภัยสูงสุด (Zero-Port Standalone Security):**
  - ทำงานในโหมด Standalone Local 100% ไม่เปิดพอร์ตใดๆ ออกสู่อินเทอร์เน็ตภายนอก ปลอดภัยสำหรับเซิร์ฟเวอร์โรงเรียน
  - การส่งข้อความเป็นแบบ Outbound HTTPS ตรงไปยัง API ของ LINE และ Telegram
- **ทำงานเป็น Background Service ได้:** มีไฟล์ `STW_TeacherCheck_Silent.exe` ไร้หน้าต่างดำรบกวน พร้อมสคริปต์เปิดตัวเองอัตโนมัติเมื่อเปิดเครื่อง (Windows Startup)

---

## 🏗 โครงสร้างสถาปัตยกรรม (Architecture)

```
┌─────────────────────────────────┐
│   เครื่องสแกนนิ้ว / ใบหน้า HIP  │
└────────────────┬────────────────┘
                 │ Auto Download (รอบ 09:00, 18:00)
                 ▼
┌─────────────────────────────────┐
│     HIP Premium Time / MSSQL    │ (Database: hiptime / Synctime)
└────────────────┬────────────────┘
                 │ Query Scans, Shifts, Leaves
                 ▼
┌─────────────────────────────────┐
│    STW_TeacherCheck (Go Core)   │ ◄─── Web Dashboard (http://localhost:8090 ในเครื่อง/วง LAN)
│  • Attendance Calculator        │
│  • Balanced Budgeting (~100)    │
│  • Realtime Scheduler Loop      │
│  • Daily Rotating Logs (30d)    │ ────► บันทึกลง logs/app-YYYY-MM-DD.log
│  • Daily Excel Exporter (30d)   │ ────► บันทึกลง reports/attendance-YYYY-MM-DD.xlsx
└────────────────┬────────────────┘
                 │ LINE Messaging API (Push / Broadcast ยิงออกข้างเดียว)
                 ▼
┌─────────────────────────────────┐
│       LINE Official Account     │ ────► ผู้บริหาร / คุณครู / ผู้ปกครอง
└─────────────────────────────────┘
```

---

## 🚀 วิธีการติดตั้งและใช้งานบน Windows 7 32-bit

### 1. ดาวน์โหลดโปรแกรม
ดาวน์โหลดโฟลเดอร์ `release-windows7` จากหน้า [Releases](https://github.com/Ahmadaduwa/hip-4.0-notify/releases) หรือ Clone Repository นี้

### 2. โครงสร้างไฟล์ในเครื่อง
```text
release-windows7/
├── STW_TeacherCheck.exe         # โปรแกรมหลัก (แบบมีหน้าต่างดำ Console สำหรับตรวจสอบ Log)
├── STW_TeacherCheck_Silent.exe  # โปรแกรมหลัก (แบบ Background Service ไร้หน้าต่าง)
├── fix_sql_connection.bat       # สคริปต์ช่วยเปิดบริการ SQL Browser และพอร์ต 1433
├── install_autostart.bat        # สคริปต์ติดตั้งให้เปิดโปรแกรมอัตโนมัติเมื่อเปิดเครื่อง
├── uninstall_autostart.bat      # สคริปต์ยกเลิกการเปิดอัตโนมัติ
├── reports/                     # โฟลเดอร์เก็บไฟล์ Excel รายงานประจำวัน (เก็บย้อนหลัง 30 วัน)
├── logs/                        # โฟลเดอร์เก็บไฟล์ Log ประจำวัน (เก็บย้อนหลัง 30 วัน)
└── data/                        # โฟลเดอร์เก็บค่าการตั้งค่า (settings.json, recipients.json)
```

### 3. เริ่มต้นใช้งานครั้งแรก
1. ดับเบิลคลิกเปิด **`STW_TeacherCheck.exe`**
2. เปิดเบราว์เซอร์ไปที่: **`http://localhost:8090`** (หรือ `http://127.0.0.1:8090`)
3. เข้าสู่ระบบด้วยค่าเริ่มต้น:
   - **Username:** `admin`
   - **Password:** `admin1234`
4. คลิกปุ่ม **"ตั้งค่าระบบ"**:
   - ระบุข้อมูลเชื่อมต่อ SQL Server (`DB Host`, `Port 1433`, `User`, `Password`, `Database: hiptime`)
   - ระบุ **LINE Channel Access Token** และ **Channel Secret**
   - ตั้งเวลาส่งรายงานประจำวัน (เช่น `09:05`) และเวลาสาย (เช่น `07:30`)
   - กด **บันทึกการตั้งค่า** (ระบบจะบันทึกลงใน `data/settings.json`)
5. กดปุ่ม **"ส่งรายงานตอนนี้"** เพื่อทดสอบยิงข้อความเข้า LINE ทันที

### 4. การเปิดใช้งานจริงแบบอัตโนมัติ (Background Mode)
1. ปิดหน้าต่าง `STW_TeacherCheck.exe`
2. ดับเบิลคลิกไฟล์ **`install_autostart.bat`** เพื่อสร้าง Shortcut ของ `STW_TeacherCheck_Silent.exe` ไปยังโฟลเดอร์ Windows Startup
3. โปรแกรมจะทำงานเงียบๆ ในพื้นหลังและเปิดตัวเองทุกครั้งที่เปิดคอมพิวเตอร์

---

## 🛠 การพัฒนาและคอมไพล์ (Development & Build)

### ข้อกำหนดระบบ (Prerequisites)
- **Go 1.20** (เวอร์ชันล่าสุดที่รองรับ Windows 7 SP1)

### คำสั่งคอมไพล์สำหรับ Windows 7 32-bit (x86):
```bash
# คอมไพล์แบบ Console
cd go-stw
GOOS=windows GOARCH=386 go build -o ../release-windows7/STW_TeacherCheck.exe .

# คอมไพล์แบบ Silent (GUI / No Console Window)
GOOS=windows GOARCH=386 go build -ldflags="-H windowsgui" -o ../release-windows7/STW_TeacherCheck_Silent.exe .
```

### การรัน Unit Tests:
```bash
cd go-stw
go test -v ./...
```

---

## 📄 License
This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
