# NgobrolYuk - Real-time Chat Application

NgobrolYuk adalah aplikasi chat real-time yang dibangun menggunakan Go (Golang) dengan Fiber framework, MongoDB sebagai database, dan WebSocket untuk komunikasi real-time.

## 🚀 Features

- ✅ User Registration & Authentication dengan JWT
- ✅ Real-time messaging menggunakan WebSocket
- ✅ Online/Offline status tracking
- ✅ Message read receipts
- ✅ User profile management
- ✅ Conversation history
- ✅ Rate limiting untuk keamanan
- ✅ HTTP-only cookies untuk token security
- ✅ Password hashing dengan bcrypt

## 🛠 Tech Stack

- **Backend**: Go (Golang) dengan Fiber Framework
- **Database**: MongoDB dengan indexing
- **Authentication**: JWT (JSON Web Tokens)
- **Real-time**: WebSocket
- **Password Hashing**: bcrypt

## 📋 Prerequisites

- Go 1.19+
- MongoDB 4.4+
- Git

## 🚀 Installation & Setup

### 1. Clone Repository

```bash
git clone https://github.com/Adisonsmn/ngobrolyuk.git
cd ngobrolyuk
```

### 2. Install Dependencies

```bash
go mod tidy
```

### 3. Environment Variables

Buat file `.env` di root directory:

```env
MONGO_URI=mongodb://localhost:27017
JWT_SECRET=your_super_secret_jwt_key_here_make_it_long_and_complex
ENVIRONMENT=development
PORT=8080
```

### 4. Run Application

```bash
go run main.go
```

Server akan berjalan di `http://localhost:8080`

## 📚 API Documentation

### Base URL

```
http://localhost:8080/api/v1
```

### Authentication Endpoints

#### 1. Register User

```http
POST /api/v1/auth/register
```

**Request Body:**

```json
{
  "username": "johndoe",
  "email": "john@example.com",
  "password": "password123"
}
```

**Response Success (201):**

```json
{
  "message": "Registration successful",
  "user": {
    "id": "1",
    "username": "johndoe",
    "email": "john@example.com",
    "bio": "",
    "avatar": ""
  }
}
```

**Response Error (400):**

```json
{
  "error": "Validation failed",
  "errors": [
    "Username must be at least 3 characters long",
    "Invalid email format"
  ]
}
```

#### 2. Login User

```http
POST /api/v1/auth/login
```

**Request Body:**

```json
{
  "email": "john@example.com",
  "password": "password123"
}
```

**Response Success (200):**

```json
{
  "message": "Login successful",
  "user": {
    "id": "1",
    "username": "johndoe",
    "email": "john@example.com",
    "bio": "",
    "avatar": ""
  }
}
```

#### 3. Logout User

```http
POST /api/v1/auth/logout
```

_Requires Authentication_

**Response (200):**

```json
{
  "message": "Logged out successfully"
}
```

#### 4. Refresh Token

```http
POST /api/v1/auth/refresh
```

_Requires Authentication_

**Response (200):**

```json
{
  "message": "Token refreshed successfully"
}
```

### User Management Endpoints

#### 1. Get Own Profile

```http
GET /api/v1/users/profile
```

_Requires Authentication_

**Response (200):**

```json
{
  "id": "1",
  "username": "johndoe",
  "email": "john@example.com",
  "bio": "Hello, I'm John!",
  "avatar": "avatar_url",
  "online": true,
  "last_seen": "2024-01-20T10:30:00Z",
  "created_at": "2024-01-15T08:00:00Z"
}
```

#### 2. Update Profile

```http
PUT /api/v1/users/profile
```

_Requires Authentication_

**Request Body:**

```json
{
  "username": "newusername",
  "bio": "Updated bio",
  "avatar": "new_avatar_url"
}
```

**Response (200):**

```json
{
  "message": "Profile updated successfully"
}
```

#### 3. List Users

```http
GET /api/v1/users?page=1&limit=20&online=true&search=john
```

_Requires Authentication_

**Query Parameters:**

- `page` (optional): Page number (default: 1)
- `limit` (optional): Items per page (default: 20, max: 100)
- `online` (optional): Filter online users (true/false)
- `search` (optional): Search by username or email

**Response (200):**

```json
{
  "pagination": {
    "limit": 20,
    "page": 1,
    "total": 3,
    "total_pages": 1
  },
  "users": [
    {
      "id": "004",
      "username": "son2",
      "bio": null,
      "avatar": null,
      "online": true,
      "last_seen": null
    },
    {
      "id": "005",
      "username": "son1",
      "bio": null,
      "avatar": null,
      "online": true,
      "last_seen": null
    },
    {
      "id": "006",
      "username": "son3",
      "bio": null,
      "avatar": null,
      "online": true,
      "last_seen": null
    }
  ]
}
```

#### 4. Get User Profile by ID

```http
GET /api/v1/users/{user_id}
```

_Requires Authentication_

**Response (200):**

```json
{
  "id": "2",
  "username": "jane",
  "bio": "Hello!",
  "avatar": "avatar_url",
  "online": true,
  "last_seen": "2024-01-20T10:25:00Z"
}
```

#### 5. Get Online Users

```http
GET /api/v1/users/online
```

_Requires Authentication_

**Response (200):**

```json
{
  "online_users": [
    {
      "id": "2",
      "username": "jane",
      "avatar": "avatar_url"
    }
  ],
  "count": 1
}
```

### Chat Endpoints

#### 1. Get Messages

```http
GET /api/v1/chat/messages?user_id=2&page=1&limit=50
```

_Requires Authentication_

**Query Parameters:**

- `user_id` (required): ID of the other user
- `page` (optional): Page number (default: 1)
- `limit` (optional): Messages per page (default: 50, max: 100)

**Response (200):**

```json
{
  "messages": [
    {
      "id": "60f7d1234567890123456789",
      "sender_id": "1",
      "receiver_id": "2",
      "content": "Hello!",
      "type": "text",
      "read": true,
      "created_at": "2024-01-20T10:30:00Z"
    }
  ],
  "pagination": {
    "page": 1,
    "limit": 50
  }
}
```

#### 2. Get Conversations

```http
GET /api/v1/chat/conversations
```

_Requires Authentication_

**Response (200):**

```json
{
  "conversations": [
    {
      "user": {
        "id": "2",
        "username": "jane",
        "avatar": "avatar_url",
        "online": true
      },
      "last_message": {
        "content": "How are you?",
        "created_at": "2024-01-20T10:30:00Z",
        "sender_id": "2"
      },
      "unread_count": 2
    }
  ]
}
```

#### 3. Mark Messages as Read

```http
PUT /api/v1/chat/read/{user_id}
```

_Requires Authentication_

**Response (200):**

```json
{
  "message": "Messages marked as read",
  "messages_updated": 5
}
```

#### 4. Get Unread Count

```http
GET /api/v1/chat/unread
```

_Requires Authentication_

**Response (200):**

```json
{
  "unread_count": 10
}
```

### WebSocket Connection

#### Connect to WebSocket

```
ws://localhost:8080/ws?token=YOUR_JWT_TOKEN
```

#### Send Message (WebSocket)

```json
{
  "receiver_id": "2",
  "content": "Hello from WebSocket!",
  "type": "text"
}
```

#### Receive Message (WebSocket)

```json
{
  "id": "60f7d1234567890123456789",
  "sender_id": "1",
  "receiver_id": "2",
  "content": "Hello from WebSocket!",
  "type": "text",
  "read": false,
  "created_at": "2024-01-20T10:30:00Z"
}
```

### Health Check

#### Check API Health

```http
GET /api/v1/health
```

**Response (200):**

```json
{
  "status": "ok",
  "timestamp": 1705744200
}
```

## 🔐 Authentication

Aplikasi menggunakan JWT (JSON Web Tokens) untuk authentication dengan HTTP-only cookies untuk keamanan tambahan.

### Cookie Configuration

- **Name**: `jwt`
- **HttpOnly**: `true`
- **Secure**: `true` (production only)
- **SameSite**: `Strict`
- **Expiration**: 72 hours

### Authorization Header (Alternative)

```http
Authorization: Bearer YOUR_JWT_TOKEN
```

## 📱 Step-by-Step Usage Guide

### 1. Setup & Registration

1. Jalankan aplikasi (`go run main.go`)
2. Buat akun baru via `POST /api/v1/auth/register`
3. Login via `POST /api/v1/auth/login`

### 2. Profile Management

1. Lihat profil sendiri: `GET /api/v1/users/profile`
2. Update profil: `PUT /api/v1/users/profile`
3. Cari user lain: `GET /api/v1/users?search=username`

### 3. Start Chatting

1. Koneksi ke WebSocket: `ws://localhost:8080/ws?token=JWT_TOKEN`
2. Kirim pesan via WebSocket atau lihat conversation history
3. Terima pesan real-time via WebSocket

### 4. Message Management

1. Lihat history chat: `GET /api/v1/chat/messages?user_id=TARGET_ID`
2. Lihat daftar conversations: `GET /api/v1/chat/conversations`
3. Mark messages sebagai read: `PUT /api/v1/chat/read/USER_ID`

## ⚠️ Rate Limiting

- **Auth endpoints**: 15 requests per 15 minutes per IP
- **WebSocket**: Max 3 connections per IP
- **General API**: No limit (tapi bisa ditambahkan sesuai kebutuhan)

## 🔒 Security Features

- Password hashing dengan bcrypt (cost: 14)
- JWT dengan expiration time
- HTTP-only cookies
- Input validation & sanitization
- Rate limiting untuk mencegah abuse
- CORS configuration
- SQL injection prevention (NoSQL injection untuk MongoDB)

## 🛠 Development

### Project Structure

```
ngobrolyuk/
├── config/          # Database & configuration
├── controllers/     # Request handlers
├── middleware/      # Authentication & rate limiting
├── models/          # Data structures & validation
├── routes/          # API routes setup
├── main.go          # Application entry point
├── go.mod           # Go dependencies
└── .env            # Environment variables
```

### Adding New Features

1. Tambah model di `models/`
2. Buat controller di `controllers/`
3. Tambah route di `routes/`
4. Test endpoint dengan Postman/curl

## 🧪 Testing

### Manual Testing dengan curl

#### Register

```bash
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "username": "testuser",
    "email": "test@example.com",
    "password": "password123"
  }'
```

#### Login

```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -c cookies.txt \
  -d '{
    "email": "test@example.com",
    "password": "password123"
  }'
```

#### Get Profile (dengan cookie)

```bash
curl -X GET http://localhost:8080/api/v1/users/profile \
  -b cookies.txt
```

## 🚀 Deployment

### Environment Variables untuk Production

```env
MONGO_URI=mongodb://your-production-mongodb-url
JWT_SECRET=your-super-secure-jwt-secret-for-production
ENVIRONMENT=production
PORT=8080
```

### Build untuk Production

```bash
go build -o ngobrolyuk main.go
./ngobrolyuk
```

## 🤝 Contributing

1. Fork repository
2. Buat feature branch (`git checkout -b feature/AmazingFeature`)
3. Commit changes (`git commit -m 'Add some AmazingFeature'`)
4. Push ke branch (`git push origin feature/AmazingFeature`)
5. Buat Pull Request

## 📄 License

This project is licensed under the MIT License.

## 📞 Support

Jika ada pertanyaan atau issue, silakan buat GitHub issue atau kontak developer.
adisonsmn07@gmail.com

---

**Happy Chatting with NgobrolYuk! 🚀💬**
