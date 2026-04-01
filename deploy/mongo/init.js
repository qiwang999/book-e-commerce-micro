db = db.getSiblingDB('bookhive');

db.createCollection('books');
db.createCollection('book_reviews');
db.createCollection('chat_histories');
db.createCollection('book_summaries');
db.createCollection('book_embeddings');

db.books.createIndex({ "title": "text", "author": "text", "description": "text" });
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

// 插入测试图书数据
db.books.insertMany([
    {
        title: "三体",
        author: "刘慈欣",
        isbn: "9787536692930",
        publisher: "重庆出版社",
        publish_date: "2008-01-01",
        price: 23.00,
        category: "科幻",
        subcategory: "硬科幻",
        description: "文化大革命如火如荼进行的同时，军方探寻外星文明的绝秘计划「红岸工程」取得了突破性进展。",
        cover_url: "http://127.0.0.1:9100/bookhive/covers/santi.jpg",
        pages: 302,
        language: "zh-CN",
        tags: ["科幻", "宇宙", "文明", "物理"],
        rating: 4.8,
        rating_count: 128900,
        created_at: new Date(),
        updated_at: new Date()
    },
    {
        title: "银河英雄传说",
        author: "田中芳树",
        isbn: "9784198600624",
        publisher: "德间书店",
        publish_date: "1982-11-01",
        price: 45.00,
        category: "科幻",
        subcategory: "太空歌剧",
        description: "描述银河帝国与自由行星同盟之间的战争，以莱因哈特和杨威利两位天才的对决为主线。",
        cover_url: "http://127.0.0.1:9100/bookhive/covers/logh.jpg",
        pages: 280,
        language: "ja",
        tags: ["科幻", "战争", "政治", "历史"],
        rating: 4.7,
        rating_count: 56000,
        created_at: new Date(),
        updated_at: new Date()
    },
    {
        title: "挪威的森林",
        author: "村上春树",
        isbn: "9784062748681",
        publisher: "讲谈社",
        publish_date: "1987-09-04",
        price: 35.00,
        category: "文学",
        subcategory: "日本文学",
        description: "描写主人公渡边彻在大学时代的爱情故事，于直子和绿子之间挣扎彷徨。",
        cover_url: "http://127.0.0.1:9100/bookhive/covers/norwegian-wood.jpg",
        pages: 296,
        language: "ja",
        tags: ["文学", "爱情", "青春", "日本"],
        rating: 4.5,
        rating_count: 89000,
        created_at: new Date(),
        updated_at: new Date()
    },
    {
        title: "Go程序设计语言",
        author: "Alan Donovan",
        isbn: "9780134190440",
        publisher: "Addison-Wesley",
        publish_date: "2015-11-01",
        price: 68.00,
        category: "技术",
        subcategory: "编程语言",
        description: "Go语言圣经，从基础到高级全面覆盖Go语言的核心概念和编程技巧。",
        cover_url: "http://127.0.0.1:9100/bookhive/covers/gopl.jpg",
        pages: 380,
        language: "en",
        tags: ["Go", "编程", "计算机", "技术"],
        rating: 4.6,
        rating_count: 12000,
        created_at: new Date(),
        updated_at: new Date()
    },
    {
        title: "海贼王 1",
        author: "尾田栄一郎",
        isbn: "9784088725093",
        publisher: "集英社",
        publish_date: "1997-12-24",
        price: 12.00,
        category: "漫画",
        subcategory: "少年漫画",
        description: "少年路飞为了实现与「红发」香克斯的约定而出海，目标是成为海贼王！",
        cover_url: "http://127.0.0.1:9100/bookhive/covers/onepiece-1.jpg",
        pages: 208,
        language: "ja",
        tags: ["漫画", "冒险", "热血", "海贼"],
        rating: 4.9,
        rating_count: 250000,
        created_at: new Date(),
        updated_at: new Date()
    },
    {
        title: "人类简史",
        author: "尤瓦尔·赫拉利",
        isbn: "9787508647357",
        publisher: "中信出版社",
        publish_date: "2014-11-01",
        price: 55.00,
        category: "历史",
        subcategory: "世界史",
        description: "从十万年前有生命迹象开始到21世纪资本、科技交织的人类发展史。",
        cover_url: "http://127.0.0.1:9100/bookhive/covers/sapiens.jpg",
        pages: 440,
        language: "zh-CN",
        tags: ["历史", "人类", "文明", "社会"],
        rating: 4.5,
        rating_count: 78000,
        created_at: new Date(),
        updated_at: new Date()
    },
    {
        title: "小王子",
        author: "安托万·德·圣-埃克苏佩里",
        isbn: "9787020042494",
        publisher: "人民文学出版社",
        publish_date: "2003-08-01",
        price: 22.00,
        category: "文学",
        subcategory: "童话",
        description: "以一位飞行员作为故事叙述者，讲述了小王子从自己星球出发前往地球的过程中的各种历险。",
        cover_url: "http://127.0.0.1:9100/bookhive/covers/little-prince.jpg",
        pages: 97,
        language: "zh-CN",
        tags: ["童话", "哲学", "经典", "儿童"],
        rating: 4.8,
        rating_count: 320000,
        created_at: new Date(),
        updated_at: new Date()
    },
    {
        title: "鬼灭之刃 1",
        author: "吾峠呼世晴",
        isbn: "9784088807232",
        publisher: "集英社",
        publish_date: "2016-06-03",
        price: 12.00,
        category: "漫画",
        subcategory: "少年漫画",
        description: "少年炭治郎为了寻找让变成鬼的妹妹恢复为人类的方法，踏上了斩鬼之旅。",
        cover_url: "http://127.0.0.1:9100/bookhive/covers/kimetsu-1.jpg",
        pages: 192,
        language: "ja",
        tags: ["漫画", "战斗", "鬼", "大正"],
        rating: 4.7,
        rating_count: 180000,
        created_at: new Date(),
        updated_at: new Date()
    }
]);
