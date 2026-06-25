# Virtual Exam API

Backend API สำหรับแพลตฟอร์มจำลองสอบเสมือนจริง (Thai-style multiple-choice exams with bubble answer sheets).

## Tech Stack

- Go 1.23
- Echo
- GORM + PostgreSQL
- Redis
- JWT Auth

## Quick Start (Docker Compose)

```bash
cd api
cp .env.example .env
docker compose up --build
```

API: `http://localhost:8080`  
Health: `GET /health`  
Base URL: `http://localhost:8080/api/v1`

## Local Development (without Docker)

1. Start PostgreSQL and Redis locally
2. Copy env file:

```bash
cp .env.example .env
```

3. Run migrations + seed + server:

```bash
go run ./cmd/server
```

Or seed only:

```bash
go run ./cmd/seed
```

## Frontend Integration

Set in Next.js:

```env
NEXT_PUBLIC_API_BASE_URL=http://localhost:8080/api/v1
```

CORS allows `http://localhost:3000` by default.

Demo exam set code for frontend: `demo`

## Demo Credentials

| Email | Password |
|-------|----------|
| demo@example.com | password123 |

## API Endpoints

| Method | Path | Auth |
|--------|------|------|
| POST | /api/v1/auth/register | No |
| POST | /api/v1/auth/login | No |
| GET | /api/v1/auth/me | Yes |
| GET | /api/v1/home | Optional |
| GET | /api/v1/exam-tracks | No |
| GET | /api/v1/exam-tracks/:trackCode | No |
| GET | /api/v1/exam-tracks/:trackCode/exam-sets | No |
| GET | /api/v1/exam-sets | No |
| GET | /api/v1/exam-sets/:examSetCode | No |
| GET | /api/v1/exam-sets/:examSetCode/questions-preview | No |
| POST | /api/v1/exam-sets/:examSetCode/attempts | Yes |
| GET | /api/v1/attempts/:attemptId | Yes |
| PUT | /api/v1/attempts/:attemptId/answers/:questionNo | Yes |
| DELETE | /api/v1/attempts/:attemptId/answers/:questionNo | Yes |
| POST | /api/v1/attempts/:attemptId/submit | Yes |
| GET | /api/v1/attempts/:attemptId/result | Yes |
| GET | /api/v1/attempts/:attemptId/review | Yes |

## Example cURL Commands

### 1. Register

```bash
curl -s -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "display_name": "Demo User",
    "email": "user@example.com",
    "password": "password123"
  }' | jq
```

### 2. Login

```bash
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "demo@example.com",
    "password": "password123"
  }' | jq -r '.data.access_token')

echo $TOKEN
```

### 3. Get Home

```bash
curl -s http://localhost:8080/api/v1/home \
  -H "Authorization: Bearer $TOKEN" | jq
```

### 4. Get Exam Tracks

```bash
curl -s http://localhost:8080/api/v1/exam-tracks | jq
```

### 5. Get Exam Sets

```bash
curl -s "http://localhost:8080/api/v1/exam-sets?access_type=free&page=1&limit=10" | jq
```

### 6. Start Attempt

```bash
ATTEMPT=$(curl -s -X POST http://localhost:8080/api/v1/exam-sets/demo/attempts \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" | jq -r '.data.attempt_id')

echo $ATTEMPT
```

### 7. Save Answer

```bash
curl -s -X PUT "http://localhost:8080/api/v1/attempts/$ATTEMPT/answers/1" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"selected_choice_key": "B"}' | jq
```

### 8. Submit Attempt

```bash
curl -s -X POST "http://localhost:8080/api/v1/attempts/$ATTEMPT/submit" \
  -H "Authorization: Bearer $TOKEN" | jq
```

### 9. Get Result

```bash
curl -s "http://localhost:8080/api/v1/attempts/$ATTEMPT/result" \
  -H "Authorization: Bearer $TOKEN" | jq
```

### 10. Get Review

```bash
curl -s "http://localhost:8080/api/v1/attempts/$ATTEMPT/review" \
  -H "Authorization: Bearer $TOKEN" | jq
```

## Project Structure

```
api/
├── cmd/server/          # HTTP server entrypoint
├── cmd/seed/            # Seed command
├── internal/            # Clean architecture modules
├── migrations/          # SQL migrations
├── seed/                # Seed logic
├── docker-compose.yml
└── Dockerfile
```

## Seed Data

- Exam tracks: สอบ ก.พ., สอบตำรวจ, สอบท้องถิ่น, สอบครูผู้ช่วย
- Exam sets: ก.พ. ชุดที่ 1/2, ตร. ชุดที่ 1/2, demo
- 20 questions per exam set (Thai content, choices ก/ข/ค/ง)
- Demo user: demo@example.com / password123

## Response Format

Success:

```json
{ "data": {} }
```

Error:

```json
{
  "error": {
    "code": "ATTEMPT_NOT_FOUND",
    "message": "ไม่พบข้อมูลการสอบ"
  }
}
```

## Notes

- PostgreSQL is the source of truth; Redis caches attempt answers during exam
- Premium payment is stubbed (premium sets return `PREMIUM_REQUIRED`)
- Leaderboard is not implemented in this phase
