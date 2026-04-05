package handler

import (
	"context"
	"fmt"
	"log"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/qiwang/book-e-commerce-micro/common/bookevent"
	pb "github.com/qiwang/book-e-commerce-micro/proto/book"
	"github.com/qiwang/book-e-commerce-micro/service/book/model"
	"github.com/qiwang/book-e-commerce-micro/service/book/repository"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type BookHandler struct {
	repo   *repository.BookRepository
	esRepo *repository.ESRepo
	mqCh   *amqp.Channel // optional: publishes book.changed when non-nil
}

func NewBookHandler(repo *repository.BookRepository, esRepo *repository.ESRepo, mqCh *amqp.Channel) *BookHandler {
	return &BookHandler{repo: repo, esRepo: esRepo, mqCh: mqCh}
}

func (h *BookHandler) GetBookDetail(ctx context.Context, req *pb.GetBookDetailRequest, rsp *pb.Book) error {
	oid, err := primitive.ObjectIDFromHex(req.BookId)
	if err != nil {
		return fmt.Errorf("invalid book id: %w", err)
	}

	book, err := h.repo.FindByID(ctx, oid)
	if err != nil {
		return fmt.Errorf("book not found: %w", err)
	}

	bookToProto(book, rsp)
	return nil
}

func (h *BookHandler) GetBooksByIds(ctx context.Context, req *pb.GetBooksByIdsRequest, rsp *pb.BookListResponse) error {
	oids := make([]primitive.ObjectID, 0, len(req.BookIds))
	for _, id := range req.BookIds {
		oid, err := primitive.ObjectIDFromHex(id)
		if err != nil {
			continue
		}
		oids = append(oids, oid)
	}

	books, err := h.repo.FindByIDs(ctx, oids)
	if err != nil {
		return fmt.Errorf("failed to fetch books: %w", err)
	}

	rsp.Books = make([]*pb.Book, 0, len(books))
	for _, b := range books {
		pbBook := &pb.Book{}
		bookToProto(b, pbBook)
		rsp.Books = append(rsp.Books, pbBook)
	}
	rsp.Total = int64(len(books))
	return nil
}

func (h *BookHandler) SearchBooks(ctx context.Context, req *pb.SearchBooksRequest, rsp *pb.BookListResponse) error {
	if h.esRepo != nil && req.Keyword != "" {
		bookIDs, total, err := h.esRepo.SearchBooks(
			ctx, req.Keyword, req.Category, req.Author,
			req.MinPrice, req.MaxPrice, req.Language,
			int(req.Page), int(req.PageSize),
		)
		if err != nil {
			log.Printf("[ES] search failed, falling back to MongoDB: %v", err)
		} else {
			oids := make([]primitive.ObjectID, 0, len(bookIDs))
			for _, id := range bookIDs {
				oid, err := primitive.ObjectIDFromHex(id)
				if err != nil {
					continue
				}
				oids = append(oids, oid)
			}

			books, err := h.repo.FindByIDs(ctx, oids)
			if err != nil {
				return fmt.Errorf("failed to fetch books by ES results: %w", err)
			}

			idxMap := make(map[string]int, len(bookIDs))
			for i, id := range bookIDs {
				idxMap[id] = i
			}
			sorted := make([]*model.Book, len(books))
			for _, b := range books {
				if idx, ok := idxMap[b.ID.Hex()]; ok {
					sorted[idx] = b
				}
			}

			rsp.Books = make([]*pb.Book, 0, len(sorted))
			for _, b := range sorted {
				if b == nil {
					continue
				}
				pbBook := &pb.Book{}
				bookToProto(b, pbBook)
				rsp.Books = append(rsp.Books, pbBook)
			}
			rsp.Total = total
			return nil
		}
	}

	books, total, err := h.repo.Search(
		ctx,
		req.Keyword, req.Category, req.Author, req.Language,
		req.MinPrice, req.MaxPrice,
		req.SortBy, req.Page, req.PageSize,
	)
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	rsp.Books = make([]*pb.Book, 0, len(books))
	for _, b := range books {
		pbBook := &pb.Book{}
		bookToProto(b, pbBook)
		rsp.Books = append(rsp.Books, pbBook)
	}
	rsp.Total = total
	return nil
}

func (h *BookHandler) ListByCategory(ctx context.Context, req *pb.ListByCategoryRequest, rsp *pb.BookListResponse) error {
	books, total, err := h.repo.ListByCategory(ctx, req.Category, req.Subcategory, req.Page, req.PageSize)
	if err != nil {
		return fmt.Errorf("list by category failed: %w", err)
	}

	rsp.Books = make([]*pb.Book, 0, len(books))
	for _, b := range books {
		pbBook := &pb.Book{}
		bookToProto(b, pbBook)
		rsp.Books = append(rsp.Books, pbBook)
	}
	rsp.Total = total
	return nil
}

func (h *BookHandler) CreateBook(ctx context.Context, req *pb.CreateBookRequest, rsp *pb.Book) error {
	if req.Title == "" || req.Author == "" || req.Category == "" {
		return fmt.Errorf("title, author and category are required")
	}
	if req.Price < 0 {
		return fmt.Errorf("price must be non-negative")
	}

	book := &model.Book{
		Title:       req.Title,
		Author:      req.Author,
		ISBN:        req.Isbn,
		Publisher:   req.Publisher,
		PublishDate: req.PublishDate,
		Price:       req.Price,
		Category:    req.Category,
		Subcategory: req.Subcategory,
		Description: req.Description,
		CoverURL:    req.CoverUrl,
		Pages:       req.Pages,
		Language:    req.Language,
		Tags:        req.Tags,
	}

	created, err := h.repo.Create(ctx, book)
	if err != nil {
		return fmt.Errorf("failed to create book: %w", err)
	}

	h.indexBookToES(ctx, created)

	bookToProto(created, rsp)
	if err := bookevent.Publish(ctx, h.mqCh, bookevent.EventCreated, rsp.Id); err != nil {
		log.Printf("[book] publish book.changed: %v", err)
	}
	return nil
}

func (h *BookHandler) UpdateBook(ctx context.Context, req *pb.UpdateBookRequest, rsp *pb.Book) error {
	oid, err := primitive.ObjectIDFromHex(req.BookId)
	if err != nil {
		return fmt.Errorf("invalid book id: %w", err)
	}

	update := bson.M{}
	if req.Title != "" {
		update["title"] = req.Title
	}
	if req.Author != "" {
		update["author"] = req.Author
	}
	if req.Price > 0 {
		update["price"] = req.Price
	}
	if req.Description != "" {
		update["description"] = req.Description
	}
	if req.CoverUrl != "" {
		update["cover_url"] = req.CoverUrl
	}
	if len(req.Tags) > 0 {
		update["tags"] = req.Tags
	}

	if len(update) == 0 {
		return fmt.Errorf("no fields to update")
	}

	updated, err := h.repo.Update(ctx, oid, update)
	if err != nil {
		return fmt.Errorf("failed to update book: %w", err)
	}

	h.indexBookToES(ctx, updated)

	bookToProto(updated, rsp)
	if err := bookevent.Publish(ctx, h.mqCh, bookevent.EventUpdated, req.BookId); err != nil {
		log.Printf("[book] publish book.changed: %v", err)
	}
	return nil
}

func (h *BookHandler) DeleteBook(ctx context.Context, req *pb.DeleteBookRequest, rsp *pb.CommonResponse) error {
	oid, err := primitive.ObjectIDFromHex(req.BookId)
	if err != nil {
		return fmt.Errorf("invalid book id: %w", err)
	}

	if err := h.repo.Delete(ctx, oid); err != nil {
		return fmt.Errorf("failed to delete book: %w", err)
	}

	if h.esRepo != nil {
		if err := h.esRepo.DeleteBook(ctx, req.BookId); err != nil {
			log.Printf("[ES] failed to delete book %s from index: %v", req.BookId, err)
		}
	}

	rsp.Code = 0
	rsp.Message = "book deleted successfully"
	return nil
}

func (h *BookHandler) ListCategories(ctx context.Context, _ *pb.ListCategoriesRequest, rsp *pb.CategoryListResponse) error {
	results, err := h.repo.ListCategories(ctx)
	if err != nil {
		return fmt.Errorf("failed to list categories: %w", err)
	}

	rsp.Categories = make([]*pb.Category, 0, len(results))
	for _, r := range results {
		rsp.Categories = append(rsp.Categories, &pb.Category{
			Name:          r.Name,
			Subcategories: r.Subcategories,
			Count:         r.Count,
		})
	}
	return nil
}

func (h *BookHandler) indexBookToES(ctx context.Context, book *model.Book) {
	if h.esRepo == nil {
		return
	}
	doc := map[string]interface{}{
		"title":        book.Title,
		"author":       book.Author,
		"description":  book.Description,
		"category":     book.Category,
		"subcategory":  book.Subcategory,
		"tags":         book.Tags,
		"isbn":         book.ISBN,
		"publisher":    book.Publisher,
		"language":     book.Language,
		"price":        book.Price,
		"rating":       book.Rating,
		"rating_count": book.RatingCount,
	}
	if err := h.esRepo.IndexBook(ctx, book.ID.Hex(), doc); err != nil {
		log.Printf("[ES] failed to index book %s: %v", book.ID.Hex(), err)
	}
}

func bookToProto(b *model.Book, pb *pb.Book) {
	pb.Id = b.ID.Hex()
	pb.Title = b.Title
	pb.Author = b.Author
	pb.Isbn = b.ISBN
	pb.Publisher = b.Publisher
	pb.PublishDate = b.PublishDate
	pb.Price = b.Price
	pb.Category = b.Category
	pb.Subcategory = b.Subcategory
	pb.Description = b.Description
	pb.CoverUrl = b.CoverURL
	pb.Pages = b.Pages
	pb.Language = b.Language
	pb.Tags = b.Tags
	pb.Rating = b.Rating
	pb.RatingCount = b.RatingCount
}
