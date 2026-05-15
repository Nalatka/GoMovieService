# NATS Events

Все события между сервисами. Используем JetStream для персистентности.

## Subjects (топики)

### user.registered
**Publisher:** User Service  
**Subscribers:** Content Service, Stream Service  
**Payload:**
```json
{
  "user_id": "uuid",
  "email": "user@example.com",
  "username": "john"
}
```

---

### user.deleted
**Publisher:** User Service  
**Subscribers:** Content Service (удалить рейтинги), Stream Service (закрыть сессии)  
**Payload:**
```json
{
  "user_id": "uuid"
}
```

---

### stream.completed
**Publisher:** Stream Service (когда StopStream и watched > 80%)  
**Subscribers:** User Service (записать в историю)  
**Payload:**
```json
{
  "user_id": "uuid",
  "movie_id": "uuid",
  "watched_seconds": 5400
}
```

---

### stream.started
**Publisher:** Stream Service (когда StartStream)  
**Subscribers:** Content Service (инкремент счётчика просмотров)  
**Payload:**
```json
{
  "movie_id": "uuid",
  "user_id": "uuid"
}
```

---

### movie.rated
**Publisher:** Content Service (когда RateMovie)  
**Subscribers:** User Service (обновить рекомендации)  
**Payload:**
```json
{
  "user_id": "uuid",
  "movie_id": "uuid",
  "score": 4.5,
  "genre_id": "uuid"
}
```
