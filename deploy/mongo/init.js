db = db.getSiblingDB('bookhive');

db.createCollection('books');
db.createCollection('book_reviews');
db.createCollection('chat_histories');
db.createCollection('book_summaries');
db.createCollection('book_embeddings');

db.books.createIndex(
    { "title": "text", "author": "text", "description": "text" },
    { default_language: "none", language_override: "_lang_unused" }
);
db.books.createIndex({ "isbn": 1 }, { unique: true, sparse: true });
db.books.createIndex({ "category": 1 });
db.books.createIndex({ "author": 1 });
db.books.createIndex({ "created_at": -1 });

db.book_reviews.createIndex({ "book_id": 1 });
db.book_reviews.createIndex({ "user_id": 1 });

db.chat_histories.createIndex({ "user_id": 1, "session_id": 1 });
db.chat_histories.createIndex({ "created_at": 1 }, { expireAfterSeconds: 2592000 });

db.book_summaries.createIndex({ "book_id": 1 }, { unique: true });

db.book_embeddings.createIndex({ "book_id": 1 }, { unique: true });

// 图书数据由 Open Library API 灌入（见 compose 中 mongo-seed-books / make seed-books-openlibrary），不在此写死样例。
