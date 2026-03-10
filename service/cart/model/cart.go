package model

type CartData struct {
	UserID  uint64     `json:"user_id"`
	StoreID uint64     `json:"store_id"`
	Items   []CartItem `json:"items"`
}

type CartItem struct {
	ItemID     string  `json:"item_id"`
	BookID     string  `json:"book_id"`
	BookTitle  string  `json:"book_title"`
	BookAuthor string  `json:"book_author"`
	BookCover  string  `json:"book_cover"`
	Price      float64 `json:"price"`
	Quantity   int32   `json:"quantity"`
	StoreID    uint64  `json:"store_id"`
}
