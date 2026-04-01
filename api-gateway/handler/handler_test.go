package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/qiwang/book-e-commerce-micro/api-gateway/handler"
	"github.com/qiwang/book-e-commerce-micro/api-gateway/router"
	"github.com/qiwang/book-e-commerce-micro/common/auth"
	aiPb "github.com/qiwang/book-e-commerce-micro/proto/ai"
	bookPb "github.com/qiwang/book-e-commerce-micro/proto/book"
	cartPb "github.com/qiwang/book-e-commerce-micro/proto/cart"
	inventoryPb "github.com/qiwang/book-e-commerce-micro/proto/inventory"
	orderPb "github.com/qiwang/book-e-commerce-micro/proto/order"
	paymentPb "github.com/qiwang/book-e-commerce-micro/proto/payment"
	storePb "github.com/qiwang/book-e-commerce-micro/proto/store"
	userPb "github.com/qiwang/book-e-commerce-micro/proto/user"
	"go-micro.dev/v4/client"
)

// ============================================================
// Mock: UserService
// ============================================================

type mockUserService struct {
	registerFn              func(context.Context, *userPb.RegisterRequest, ...client.CallOption) (*userPb.AuthResponse, error)
	loginFn                 func(context.Context, *userPb.LoginRequest, ...client.CallOption) (*userPb.AuthResponse, error)
	sendVerificationCodeFn  func(context.Context, *userPb.SendCodeRequest, ...client.CallOption) (*userPb.CommonResponse, error)
	getProfileFn            func(context.Context, *userPb.GetProfileRequest, ...client.CallOption) (*userPb.UserProfile, error)
	updateProfileFn         func(context.Context, *userPb.UpdateProfileRequest, ...client.CallOption) (*userPb.CommonResponse, error)
	getPreferencesFn        func(context.Context, *userPb.GetPreferencesRequest, ...client.CallOption) (*userPb.UserPreferences, error)
	updatePreferencesFn     func(context.Context, *userPb.UpdatePreferencesRequest, ...client.CallOption) (*userPb.CommonResponse, error)
}

func (m *mockUserService) Register(ctx context.Context, req *userPb.RegisterRequest, opts ...client.CallOption) (*userPb.AuthResponse, error) {
	if m.registerFn != nil {
		return m.registerFn(ctx, req, opts...)
	}
	return &userPb.AuthResponse{Token: "mock-token", User: &userPb.UserInfo{Id: 1, Email: req.Email, Role: "user"}}, nil
}
func (m *mockUserService) SendVerificationCode(ctx context.Context, req *userPb.SendCodeRequest, opts ...client.CallOption) (*userPb.CommonResponse, error) {
	if m.sendVerificationCodeFn != nil {
		return m.sendVerificationCodeFn(ctx, req, opts...)
	}
	return &userPb.CommonResponse{Code: 200, Message: "verification code sent"}, nil
}
func (m *mockUserService) Login(ctx context.Context, req *userPb.LoginRequest, opts ...client.CallOption) (*userPb.AuthResponse, error) {
	if m.loginFn != nil {
		return m.loginFn(ctx, req, opts...)
	}
	return &userPb.AuthResponse{Token: "mock-token", User: &userPb.UserInfo{Id: 1, Email: req.Email, Role: "user"}}, nil
}
func (m *mockUserService) GetProfile(ctx context.Context, req *userPb.GetProfileRequest, opts ...client.CallOption) (*userPb.UserProfile, error) {
	if m.getProfileFn != nil {
		return m.getProfileFn(ctx, req, opts...)
	}
	return &userPb.UserProfile{UserId: req.UserId, Name: "Test User", Email: "test@example.com"}, nil
}
func (m *mockUserService) UpdateProfile(ctx context.Context, req *userPb.UpdateProfileRequest, opts ...client.CallOption) (*userPb.CommonResponse, error) {
	if m.updateProfileFn != nil {
		return m.updateProfileFn(ctx, req, opts...)
	}
	return &userPb.CommonResponse{Code: 0, Message: "success"}, nil
}
func (m *mockUserService) GetUserPreferences(ctx context.Context, req *userPb.GetPreferencesRequest, opts ...client.CallOption) (*userPb.UserPreferences, error) {
	if m.getPreferencesFn != nil {
		return m.getPreferencesFn(ctx, req, opts...)
	}
	return &userPb.UserPreferences{UserId: req.UserId, FavoriteCategories: []string{"fiction"}}, nil
}
func (m *mockUserService) UpdateUserPreferences(ctx context.Context, req *userPb.UpdatePreferencesRequest, opts ...client.CallOption) (*userPb.CommonResponse, error) {
	if m.updatePreferencesFn != nil {
		return m.updatePreferencesFn(ctx, req, opts...)
	}
	return &userPb.CommonResponse{Code: 0, Message: "success"}, nil
}
func (m *mockUserService) GetAddress(ctx context.Context, req *userPb.GetAddressRequest, opts ...client.CallOption) (*userPb.Address, error) {
	return &userPb.Address{Id: req.AddressId, Name: "Home"}, nil
}
func (m *mockUserService) ListAddresses(ctx context.Context, req *userPb.ListAddressesRequest, opts ...client.CallOption) (*userPb.AddressListResponse, error) {
	return &userPb.AddressListResponse{Addresses: []*userPb.Address{{Id: 1, Name: "Home"}}}, nil
}
func (m *mockUserService) CreateAddress(ctx context.Context, req *userPb.CreateAddressRequest, opts ...client.CallOption) (*userPb.Address, error) {
	return &userPb.Address{Id: 1, UserId: req.UserId, Name: req.Name, City: req.City}, nil
}
func (m *mockUserService) ValidateToken(ctx context.Context, req *userPb.ValidateTokenRequest, opts ...client.CallOption) (*userPb.ValidateTokenResponse, error) {
	return &userPb.ValidateTokenResponse{Valid: true, UserId: 1, Role: "user"}, nil
}

// ============================================================
// Mock: BookService
// ============================================================

type mockBookService struct{}

func (m *mockBookService) GetBookDetail(ctx context.Context, req *bookPb.GetBookDetailRequest, opts ...client.CallOption) (*bookPb.Book, error) {
	return &bookPb.Book{Id: req.BookId, Title: "Go Programming", Author: "Test"}, nil
}
func (m *mockBookService) GetBooksByIds(ctx context.Context, req *bookPb.GetBooksByIdsRequest, opts ...client.CallOption) (*bookPb.BookListResponse, error) {
	return &bookPb.BookListResponse{Books: []*bookPb.Book{}, Total: 0}, nil
}
func (m *mockBookService) SearchBooks(ctx context.Context, req *bookPb.SearchBooksRequest, opts ...client.CallOption) (*bookPb.BookListResponse, error) {
	return &bookPb.BookListResponse{Books: []*bookPb.Book{{Id: "b1", Title: "Go"}}, Total: 1}, nil
}
func (m *mockBookService) ListByCategory(ctx context.Context, req *bookPb.ListByCategoryRequest, opts ...client.CallOption) (*bookPb.BookListResponse, error) {
	return &bookPb.BookListResponse{Books: []*bookPb.Book{}, Total: 0}, nil
}
func (m *mockBookService) CreateBook(ctx context.Context, req *bookPb.CreateBookRequest, opts ...client.CallOption) (*bookPb.Book, error) {
	return &bookPb.Book{Id: "new-1", Title: req.Title, Author: req.Author}, nil
}
func (m *mockBookService) UpdateBook(ctx context.Context, req *bookPb.UpdateBookRequest, opts ...client.CallOption) (*bookPb.Book, error) {
	return &bookPb.Book{Id: req.BookId}, nil
}
func (m *mockBookService) DeleteBook(ctx context.Context, req *bookPb.DeleteBookRequest, opts ...client.CallOption) (*bookPb.CommonResponse, error) {
	return &bookPb.CommonResponse{Code: 0, Message: "deleted"}, nil
}
func (m *mockBookService) ListCategories(ctx context.Context, req *bookPb.ListCategoriesRequest, opts ...client.CallOption) (*bookPb.CategoryListResponse, error) {
	return &bookPb.CategoryListResponse{Categories: []*bookPb.Category{{Name: "Fiction", Count: 42}}}, nil
}

// ============================================================
// Mock: StoreService
// ============================================================

type mockStoreService struct{}

func (m *mockStoreService) ListStores(ctx context.Context, req *storePb.ListStoresRequest, opts ...client.CallOption) (*storePb.StoreListResponse, error) {
	return &storePb.StoreListResponse{Stores: []*storePb.Store{{Id: 1, Name: "Downtown"}}, Total: 1}, nil
}
func (m *mockStoreService) GetStoreDetail(ctx context.Context, req *storePb.GetStoreDetailRequest, opts ...client.CallOption) (*storePb.Store, error) {
	return &storePb.Store{Id: req.StoreId, Name: "Downtown Store"}, nil
}
func (m *mockStoreService) GetNearestStore(ctx context.Context, req *storePb.GetNearestStoreRequest, opts ...client.CallOption) (*storePb.Store, error) {
	return &storePb.Store{Id: 1, Name: "Nearest"}, nil
}
func (m *mockStoreService) GetStoresInRadius(ctx context.Context, req *storePb.GetStoresInRadiusRequest, opts ...client.CallOption) (*storePb.StoreListResponse, error) {
	return &storePb.StoreListResponse{Stores: []*storePb.Store{{Id: 1}}, Total: 1}, nil
}
func (m *mockStoreService) CreateStore(ctx context.Context, req *storePb.CreateStoreRequest, opts ...client.CallOption) (*storePb.Store, error) {
	return &storePb.Store{Id: 1, Name: req.Name}, nil
}
func (m *mockStoreService) UpdateStore(ctx context.Context, req *storePb.UpdateStoreRequest, opts ...client.CallOption) (*storePb.Store, error) {
	return &storePb.Store{Id: req.Id, Name: req.Name}, nil
}

// ============================================================
// Mock: InventoryService
// ============================================================

type mockInventoryService struct{}

func (m *mockInventoryService) CheckStock(ctx context.Context, req *inventoryPb.CheckStockRequest, opts ...client.CallOption) (*inventoryPb.StockInfo, error) {
	return &inventoryPb.StockInfo{StoreId: req.StoreId, BookId: req.BookId, Quantity: 10, Available: 10}, nil
}
func (m *mockInventoryService) BatchCheckStock(ctx context.Context, req *inventoryPb.BatchCheckStockRequest, opts ...client.CallOption) (*inventoryPb.BatchStockResponse, error) {
	return &inventoryPb.BatchStockResponse{}, nil
}
func (m *mockInventoryService) SetStock(ctx context.Context, req *inventoryPb.SetStockRequest, opts ...client.CallOption) (*inventoryPb.CommonResponse, error) {
	return &inventoryPb.CommonResponse{}, nil
}
func (m *mockInventoryService) LockStock(ctx context.Context, req *inventoryPb.LockStockRequest, opts ...client.CallOption) (*inventoryPb.LockStockResponse, error) {
	return &inventoryPb.LockStockResponse{}, nil
}
func (m *mockInventoryService) ReleaseStock(ctx context.Context, req *inventoryPb.ReleaseStockRequest, opts ...client.CallOption) (*inventoryPb.CommonResponse, error) {
	return &inventoryPb.CommonResponse{}, nil
}
func (m *mockInventoryService) DeductStock(ctx context.Context, req *inventoryPb.DeductStockRequest, opts ...client.CallOption) (*inventoryPb.CommonResponse, error) {
	return &inventoryPb.CommonResponse{}, nil
}
func (m *mockInventoryService) GetStoreBooks(ctx context.Context, req *inventoryPb.GetStoreBooksRequest, opts ...client.CallOption) (*inventoryPb.StoreBookListResponse, error) {
	return &inventoryPb.StoreBookListResponse{Items: []*inventoryPb.StoreBookItem{{BookId: "b1", Quantity: 5}}, Total: 1}, nil
}

// ============================================================
// Mock: CartService
// ============================================================

type mockCartService struct{}

func (m *mockCartService) GetCart(ctx context.Context, req *cartPb.GetCartRequest, opts ...client.CallOption) (*cartPb.Cart, error) {
	return &cartPb.Cart{UserId: req.UserId, Items: []*cartPb.CartItem{{ItemId: "item-1", BookId: "b1", Quantity: 2}}}, nil
}
func (m *mockCartService) AddToCart(ctx context.Context, req *cartPb.AddToCartRequest, opts ...client.CallOption) (*cartPb.Cart, error) {
	return &cartPb.Cart{UserId: req.UserId, Items: []*cartPb.CartItem{{BookId: req.BookId, Quantity: req.Quantity}}}, nil
}
func (m *mockCartService) RemoveFromCart(ctx context.Context, req *cartPb.RemoveFromCartRequest, opts ...client.CallOption) (*cartPb.Cart, error) {
	return &cartPb.Cart{UserId: req.UserId, Items: []*cartPb.CartItem{}}, nil
}
func (m *mockCartService) UpdateCartItem(ctx context.Context, req *cartPb.UpdateCartItemRequest, opts ...client.CallOption) (*cartPb.Cart, error) {
	return &cartPb.Cart{UserId: req.UserId, Items: []*cartPb.CartItem{{ItemId: req.ItemId, Quantity: req.Quantity}}}, nil
}
func (m *mockCartService) ClearCart(ctx context.Context, req *cartPb.ClearCartRequest, opts ...client.CallOption) (*cartPb.CommonResponse, error) {
	return &cartPb.CommonResponse{Code: 0, Message: "cleared"}, nil
}

// ============================================================
// Mock: OrderService
// ============================================================

type mockOrderService struct{}

func (m *mockOrderService) CreateOrder(ctx context.Context, req *orderPb.CreateOrderRequest, opts ...client.CallOption) (*orderPb.Order, error) {
	return &orderPb.Order{Id: 1001, OrderNo: "ORD-001", Status: "pending"}, nil
}
func (m *mockOrderService) GetOrder(ctx context.Context, req *orderPb.GetOrderRequest, opts ...client.CallOption) (*orderPb.Order, error) {
	return &orderPb.Order{Id: req.OrderId, OrderNo: req.OrderNo, Status: "pending"}, nil
}
func (m *mockOrderService) ListOrders(ctx context.Context, req *orderPb.ListOrdersRequest, opts ...client.CallOption) (*orderPb.OrderListResponse, error) {
	return &orderPb.OrderListResponse{Orders: []*orderPb.Order{{Id: 1001, OrderNo: "ORD-001", Status: "pending"}}, Total: 1}, nil
}
func (m *mockOrderService) CancelOrder(ctx context.Context, req *orderPb.CancelOrderRequest, opts ...client.CallOption) (*orderPb.CommonResponse, error) {
	return &orderPb.CommonResponse{Code: 0, Message: "cancelled"}, nil
}
func (m *mockOrderService) UpdateOrderStatus(ctx context.Context, req *orderPb.UpdateOrderStatusRequest, opts ...client.CallOption) (*orderPb.CommonResponse, error) {
	return &orderPb.CommonResponse{Code: 0, Message: "updated"}, nil
}

// ============================================================
// Mock: PaymentService
// ============================================================

type mockPaymentService struct{}

func (m *mockPaymentService) CreatePayment(ctx context.Context, req *paymentPb.CreatePaymentRequest, opts ...client.CallOption) (*paymentPb.Payment, error) {
	return &paymentPb.Payment{PaymentNo: "PAY-001", Status: "pending"}, nil
}
func (m *mockPaymentService) ProcessPayment(ctx context.Context, req *paymentPb.ProcessPaymentRequest, opts ...client.CallOption) (*paymentPb.PaymentResult, error) {
	return &paymentPb.PaymentResult{Code: 0, Message: "success", Payment: &paymentPb.Payment{PaymentNo: req.PaymentNo, Status: "paid"}}, nil
}
func (m *mockPaymentService) GetPaymentStatus(ctx context.Context, req *paymentPb.GetPaymentStatusRequest, opts ...client.CallOption) (*paymentPb.Payment, error) {
	return &paymentPb.Payment{PaymentNo: req.PaymentNo, Status: "paid"}, nil
}
func (m *mockPaymentService) RefundPayment(ctx context.Context, req *paymentPb.RefundPaymentRequest, opts ...client.CallOption) (*paymentPb.PaymentResult, error) {
	return &paymentPb.PaymentResult{Code: 0, Message: "refunded", Payment: &paymentPb.Payment{PaymentNo: req.PaymentNo, Status: "refunded"}}, nil
}
func (m *mockPaymentService) GetPaymentByOrderId(ctx context.Context, req *paymentPb.GetPaymentByOrderIdRequest, opts ...client.CallOption) (*paymentPb.Payment, error) {
	return &paymentPb.Payment{PaymentNo: "PAY-001", Status: "paid"}, nil
}

// ============================================================
// Mock: AIService
// ============================================================

type mockAIService struct{}

func (m *mockAIService) GetRecommendations(ctx context.Context, req *aiPb.RecommendRequest, opts ...client.CallOption) (*aiPb.RecommendResponse, error) {
	return &aiPb.RecommendResponse{Recommendations: []*aiPb.BookRecommendation{{BookId: "b1", Title: "Go Programming", Reason: "Based on your taste"}}}, nil
}
func (m *mockAIService) ChatWithLibrarian(ctx context.Context, req *aiPb.ChatRequest, opts ...client.CallOption) (*aiPb.ChatResponse, error) {
	return &aiPb.ChatResponse{Reply: "I recommend this book!", SessionId: req.SessionId}, nil
}
func (m *mockAIService) StreamChat(ctx context.Context, req *aiPb.ChatRequest, opts ...client.CallOption) (aiPb.AIService_StreamChatService, error) {
	return nil, fmt.Errorf("streaming not supported in mock")
}
func (m *mockAIService) GenerateBookSummary(ctx context.Context, req *aiPb.SummaryRequest, opts ...client.CallOption) (*aiPb.SummaryResponse, error) {
	return &aiPb.SummaryResponse{BookId: req.BookId, Summary: "A great book."}, nil
}
func (m *mockAIService) SmartSearch(ctx context.Context, req *aiPb.SmartSearchRequest, opts ...client.CallOption) (*aiPb.SmartSearchResponse, error) {
	return &aiPb.SmartSearchResponse{InterpretedQuery: "Found results"}, nil
}
func (m *mockAIService) AnalyzeReadingTaste(ctx context.Context, req *aiPb.TasteRequest, opts ...client.CallOption) (*aiPb.TasteResponse, error) {
	return &aiPb.TasteResponse{TasteSummary: "You love sci-fi"}, nil
}
func (m *mockAIService) GetSimilarBooks(ctx context.Context, req *aiPb.SimilarBooksRequest, opts ...client.CallOption) (*aiPb.SimilarBooksResponse, error) {
	return &aiPb.SimilarBooksResponse{}, nil
}

// ============================================================
// Helpers
// ============================================================

const jwtSecret = "test-secret"

func setupTestRouter() (*httptest.Server, *auth.JWTManager) {
	jwtMgr := auth.NewJWTManager(jwtSecret, 72)
	h := &handler.Handlers{
		User:      &mockUserService{},
		Book:      &mockBookService{},
		Store:     &mockStoreService{},
		Inventory: &mockInventoryService{},
		Cart:      &mockCartService{},
		Order:     &mockOrderService{},
		Payment:   &mockPaymentService{},
		AI:        &mockAIService{},
	}
	r := router.SetupRouter(h, jwtMgr)
	return httptest.NewServer(r), jwtMgr
}

func genToken(jwtMgr *auth.JWTManager, uid uint64, email, role string) string {
	t, _ := jwtMgr.GenerateToken(uid, email, role)
	return t
}

type apiResp struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func doReq(t *testing.T, method, url, token string, body interface{}) (*http.Response, apiResp) {
	t.Helper()
	var buf *bytes.Buffer
	if body != nil {
		b, _ := json.Marshal(body)
		buf = bytes.NewBuffer(b)
	} else {
		buf = bytes.NewBuffer(nil)
	}
	req, err := http.NewRequest(method, url, buf)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()
	var r apiResp
	json.NewDecoder(resp.Body).Decode(&r)
	return resp, r
}

func assertStatus(t *testing.T, resp *http.Response, want int) {
	t.Helper()
	if resp.StatusCode != want {
		t.Fatalf("status: want %d, got %d", want, resp.StatusCode)
	}
}

func assertCode(t *testing.T, r apiResp, want int) {
	t.Helper()
	if r.Code != want {
		t.Fatalf("code: want %d, got %d (msg=%s)", want, r.Code, r.Message)
	}
}

// ============================================================
// Health
// ============================================================

func TestHealth(t *testing.T) {
	ts, _ := setupTestRouter()
	defer ts.Close()
	resp, r := doReq(t, "GET", ts.URL+"/health", "", nil)
	assertStatus(t, resp, 200)
	assertCode(t, r, 0)
}

// ============================================================
// Auth: Register / Login
// ============================================================

func TestRegister_OK(t *testing.T) {
	ts, _ := setupTestRouter()
	defer ts.Close()
	resp, r := doReq(t, "POST", ts.URL+"/api/v1/auth/register", "", map[string]string{
		"email": "a@b.com", "password": "123456", "name": "Test",
	})
	assertStatus(t, resp, 200)
	assertCode(t, r, 0)
}

func TestRegister_EmptyBody(t *testing.T) {
	ts, _ := setupTestRouter()
	defer ts.Close()
	resp, r := doReq(t, "POST", ts.URL+"/api/v1/auth/register", "", nil)
	assertStatus(t, resp, 400)
	assertCode(t, r, 400)
}

func TestLogin_OK(t *testing.T) {
	ts, _ := setupTestRouter()
	defer ts.Close()
	resp, r := doReq(t, "POST", ts.URL+"/api/v1/auth/login", "", map[string]string{
		"email": "a@b.com", "password": "123456",
	})
	assertStatus(t, resp, 200)
	assertCode(t, r, 0)
}

func TestLogin_EmptyBody(t *testing.T) {
	ts, _ := setupTestRouter()
	defer ts.Close()
	resp, r := doReq(t, "POST", ts.URL+"/api/v1/auth/login", "", nil)
	assertStatus(t, resp, 400)
	assertCode(t, r, 400)
}

// ============================================================
// User: Profile / Preferences / Addresses (protected)
// ============================================================

func TestGetProfile_NoAuth(t *testing.T) {
	ts, _ := setupTestRouter()
	defer ts.Close()
	resp, r := doReq(t, "GET", ts.URL+"/api/v1/user/profile", "", nil)
	assertStatus(t, resp, 401)
	assertCode(t, r, 401)
}

func TestGetProfile_OK(t *testing.T) {
	ts, jwt := setupTestRouter()
	defer ts.Close()
	resp, r := doReq(t, "GET", ts.URL+"/api/v1/user/profile", genToken(jwt, 1, "a@b.com", "user"), nil)
	assertStatus(t, resp, 200)
	assertCode(t, r, 0)
}

func TestUpdateProfile_OK(t *testing.T) {
	ts, jwt := setupTestRouter()
	defer ts.Close()
	resp, r := doReq(t, "PUT", ts.URL+"/api/v1/user/profile", genToken(jwt, 1, "a@b.com", "user"),
		map[string]string{"name": "New Name", "phone": "13800000000"})
	assertStatus(t, resp, 200)
	assertCode(t, r, 0)
}

func TestGetPreferences_OK(t *testing.T) {
	ts, jwt := setupTestRouter()
	defer ts.Close()
	resp, r := doReq(t, "GET", ts.URL+"/api/v1/user/preferences", genToken(jwt, 1, "a@b.com", "user"), nil)
	assertStatus(t, resp, 200)
	assertCode(t, r, 0)
}

func TestUpdatePreferences_OK(t *testing.T) {
	ts, jwt := setupTestRouter()
	defer ts.Close()
	resp, r := doReq(t, "PUT", ts.URL+"/api/v1/user/preferences", genToken(jwt, 1, "a@b.com", "user"),
		map[string]interface{}{"favorite_categories": []string{"sci-fi"}})
	assertStatus(t, resp, 200)
	assertCode(t, r, 0)
}

func TestListAddresses_OK(t *testing.T) {
	ts, jwt := setupTestRouter()
	defer ts.Close()
	resp, r := doReq(t, "GET", ts.URL+"/api/v1/user/addresses", genToken(jwt, 1, "a@b.com", "user"), nil)
	assertStatus(t, resp, 200)
	assertCode(t, r, 0)
}

func TestCreateAddress_OK(t *testing.T) {
	ts, jwt := setupTestRouter()
	defer ts.Close()
	resp, r := doReq(t, "POST", ts.URL+"/api/v1/user/addresses", genToken(jwt, 1, "a@b.com", "user"),
		map[string]interface{}{
			"name": "张三", "phone": "13800138000", "province": "广东省",
			"city": "深圳市", "district": "南山区", "detail": "科技园路1号", "is_default": true,
		})
	assertStatus(t, resp, 200)
	assertCode(t, r, 0)
}

func TestCreateAddress_MissingFields(t *testing.T) {
	ts, jwt := setupTestRouter()
	defer ts.Close()
	resp, r := doReq(t, "POST", ts.URL+"/api/v1/user/addresses", genToken(jwt, 1, "a@b.com", "user"),
		map[string]string{"name": "张三"})
	assertStatus(t, resp, 400)
	assertCode(t, r, 400)
}

// ============================================================
// Books: Search / Detail / Categories / Create(admin)
// ============================================================

func TestSearchBooks_OK(t *testing.T) {
	ts, _ := setupTestRouter()
	defer ts.Close()
	resp, r := doReq(t, "GET", ts.URL+"/api/v1/books/search?keyword=go&page=1&page_size=10", "", nil)
	assertStatus(t, resp, 200)
	assertCode(t, r, 0)
}

func TestGetBookDetail_OK(t *testing.T) {
	ts, _ := setupTestRouter()
	defer ts.Close()
	resp, r := doReq(t, "GET", ts.URL+"/api/v1/books/book-123", "", nil)
	assertStatus(t, resp, 200)
	assertCode(t, r, 0)
}

func TestListCategories_OK(t *testing.T) {
	ts, _ := setupTestRouter()
	defer ts.Close()
	resp, r := doReq(t, "GET", ts.URL+"/api/v1/books/categories", "", nil)
	assertStatus(t, resp, 200)
	assertCode(t, r, 0)
}

func TestCreateBook_Admin_OK(t *testing.T) {
	ts, jwt := setupTestRouter()
	defer ts.Close()
	resp, r := doReq(t, "POST", ts.URL+"/api/v1/books", genToken(jwt, 1, "admin@b.com", "admin"),
		map[string]interface{}{"title": "New Book", "author": "Author", "isbn": "978-0-00-000000-0", "price": 29.99, "category": "tech"})
	assertStatus(t, resp, 200)
	assertCode(t, r, 0)
}

func TestCreateBook_Forbidden(t *testing.T) {
	ts, jwt := setupTestRouter()
	defer ts.Close()
	resp, r := doReq(t, "POST", ts.URL+"/api/v1/books", genToken(jwt, 1, "user@b.com", "user"),
		map[string]interface{}{"title": "Book"})
	assertStatus(t, resp, 403)
	assertCode(t, r, 403)
}

func TestUploadBookCover_NonAdminForbidden(t *testing.T) {
	ts, jwt := setupTestRouter()
	defer ts.Close()
	resp, r := doReq(t, "POST", ts.URL+"/api/v1/books/upload-cover", genToken(jwt, 1, "user@b.com", "user"), nil)
	assertStatus(t, resp, 403)
	assertCode(t, r, 403)
}

func TestUploadBookCover_Admin_StorageNotConfigured(t *testing.T) {
	ts, jwt := setupTestRouter()
	defer ts.Close()
	resp, r := doReq(t, "POST", ts.URL+"/api/v1/books/upload-cover", genToken(jwt, 1, "admin@b.com", "admin"), nil)
	assertStatus(t, resp, 500)
	assertCode(t, r, 500)
}

func TestCreateBook_NoAuth(t *testing.T) {
	ts, _ := setupTestRouter()
	defer ts.Close()
	resp, _ := doReq(t, "POST", ts.URL+"/api/v1/books", "", map[string]interface{}{"title": "X"})
	assertStatus(t, resp, 401)
}

// ============================================================
// Stores: List / Detail / Nearest / Radius
// ============================================================

func TestListStores_OK(t *testing.T) {
	ts, _ := setupTestRouter()
	defer ts.Close()
	resp, r := doReq(t, "GET", ts.URL+"/api/v1/stores?city=shenzhen", "", nil)
	assertStatus(t, resp, 200)
	assertCode(t, r, 0)
}

func TestGetStoreDetail_OK(t *testing.T) {
	ts, _ := setupTestRouter()
	defer ts.Close()
	resp, r := doReq(t, "GET", ts.URL+"/api/v1/stores/1", "", nil)
	assertStatus(t, resp, 200)
	assertCode(t, r, 0)
}

func TestGetStoreDetail_InvalidID(t *testing.T) {
	ts, _ := setupTestRouter()
	defer ts.Close()
	resp, r := doReq(t, "GET", ts.URL+"/api/v1/stores/abc", "", nil)
	assertStatus(t, resp, 400)
	assertCode(t, r, 400)
}

func TestGetNearestStore_OK(t *testing.T) {
	ts, _ := setupTestRouter()
	defer ts.Close()
	resp, r := doReq(t, "GET", ts.URL+"/api/v1/stores/nearest?lat=22.54&lng=114.05", "", nil)
	assertStatus(t, resp, 200)
	assertCode(t, r, 0)
}

func TestGetNearestStore_InvalidLat(t *testing.T) {
	ts, _ := setupTestRouter()
	defer ts.Close()
	resp, r := doReq(t, "GET", ts.URL+"/api/v1/stores/nearest?lat=999&lng=114", "", nil)
	assertStatus(t, resp, 400)
	assertCode(t, r, 400)
}

func TestGetStoresInRadius_OK(t *testing.T) {
	ts, _ := setupTestRouter()
	defer ts.Close()
	resp, r := doReq(t, "GET", ts.URL+"/api/v1/stores/radius?lat=22.54&lng=114.05&radius=10&limit=5", "", nil)
	assertStatus(t, resp, 200)
	assertCode(t, r, 0)
}

func TestGetStoresInRadius_BadRadius(t *testing.T) {
	ts, _ := setupTestRouter()
	defer ts.Close()
	resp, r := doReq(t, "GET", ts.URL+"/api/v1/stores/radius?lat=22.54&lng=114.05&radius=999", "", nil)
	assertStatus(t, resp, 400)
	assertCode(t, r, 400)
}

// ============================================================
// Inventory: CheckStock / StoreBooks
// ============================================================

func TestCheckStock_OK(t *testing.T) {
	ts, _ := setupTestRouter()
	defer ts.Close()
	resp, r := doReq(t, "GET", ts.URL+"/api/v1/inventory/stock?store_id=1&book_id=b1", "", nil)
	assertStatus(t, resp, 200)
	assertCode(t, r, 0)
}

func TestCheckStock_MissingStoreID(t *testing.T) {
	ts, _ := setupTestRouter()
	defer ts.Close()
	resp, r := doReq(t, "GET", ts.URL+"/api/v1/inventory/stock?book_id=b1", "", nil)
	assertStatus(t, resp, 400)
	assertCode(t, r, 400)
}

func TestCheckStock_MissingBookID(t *testing.T) {
	ts, _ := setupTestRouter()
	defer ts.Close()
	resp, r := doReq(t, "GET", ts.URL+"/api/v1/inventory/stock?store_id=1", "", nil)
	assertStatus(t, resp, 400)
	assertCode(t, r, 400)
}

func TestGetStoreBooks_OK(t *testing.T) {
	ts, _ := setupTestRouter()
	defer ts.Close()
	resp, r := doReq(t, "GET", ts.URL+"/api/v1/inventory/store/1/books?page=1&page_size=10", "", nil)
	assertStatus(t, resp, 200)
	assertCode(t, r, 0)
}

// ============================================================
// Cart: CRUD + Clear (protected)
// ============================================================

func TestGetCart_OK(t *testing.T) {
	ts, jwt := setupTestRouter()
	defer ts.Close()
	resp, r := doReq(t, "GET", ts.URL+"/api/v1/cart", genToken(jwt, 1, "a@b.com", "user"), nil)
	assertStatus(t, resp, 200)
	assertCode(t, r, 0)
}

func TestGetCart_NoAuth(t *testing.T) {
	ts, _ := setupTestRouter()
	defer ts.Close()
	resp, _ := doReq(t, "GET", ts.URL+"/api/v1/cart", "", nil)
	assertStatus(t, resp, 401)
}

func TestAddToCart_OK(t *testing.T) {
	ts, jwt := setupTestRouter()
	defer ts.Close()
	resp, r := doReq(t, "POST", ts.URL+"/api/v1/cart/items", genToken(jwt, 1, "a@b.com", "user"),
		map[string]interface{}{"store_id": 1, "book_id": "b1", "quantity": 2})
	assertStatus(t, resp, 200)
	assertCode(t, r, 0)
}

func TestRemoveFromCart_OK(t *testing.T) {
	ts, jwt := setupTestRouter()
	defer ts.Close()
	resp, r := doReq(t, "DELETE", ts.URL+"/api/v1/cart/items/item-1", genToken(jwt, 1, "a@b.com", "user"), nil)
	assertStatus(t, resp, 200)
	assertCode(t, r, 0)
}

func TestUpdateCartItem_OK(t *testing.T) {
	ts, jwt := setupTestRouter()
	defer ts.Close()
	resp, r := doReq(t, "PUT", ts.URL+"/api/v1/cart/items/item-1", genToken(jwt, 1, "a@b.com", "user"),
		map[string]interface{}{"quantity": 5})
	assertStatus(t, resp, 200)
	assertCode(t, r, 0)
}

func TestClearCart_OK(t *testing.T) {
	ts, jwt := setupTestRouter()
	defer ts.Close()
	resp, r := doReq(t, "DELETE", ts.URL+"/api/v1/cart", genToken(jwt, 1, "a@b.com", "user"), nil)
	assertStatus(t, resp, 200)
	assertCode(t, r, 0)
}

// ============================================================
// Orders: Create / List / Get / Cancel (protected)
// ============================================================

func TestCreateOrder_OK(t *testing.T) {
	ts, jwt := setupTestRouter()
	defer ts.Close()
	resp, r := doReq(t, "POST", ts.URL+"/api/v1/orders", genToken(jwt, 1, "a@b.com", "user"),
		map[string]interface{}{
			"store_id":      1,
			"items":         []map[string]interface{}{{"book_id": "b1", "book_title": "Go", "price": 29.99, "quantity": 1}},
			"pickup_method": "self_pickup",
		})
	assertStatus(t, resp, 200)
	assertCode(t, r, 0)
}

func TestListOrders_OK(t *testing.T) {
	ts, jwt := setupTestRouter()
	defer ts.Close()
	resp, r := doReq(t, "GET", ts.URL+"/api/v1/orders?page=1&page_size=10", genToken(jwt, 1, "a@b.com", "user"), nil)
	assertStatus(t, resp, 200)
	assertCode(t, r, 0)
}

func TestGetOrder_ByNumericID(t *testing.T) {
	ts, jwt := setupTestRouter()
	defer ts.Close()
	resp, r := doReq(t, "GET", ts.URL+"/api/v1/orders/1001", genToken(jwt, 1, "a@b.com", "user"), nil)
	assertStatus(t, resp, 200)
	assertCode(t, r, 0)
}

func TestGetOrder_ByOrderNo(t *testing.T) {
	ts, jwt := setupTestRouter()
	defer ts.Close()
	resp, r := doReq(t, "GET", ts.URL+"/api/v1/orders/ORD-001", genToken(jwt, 1, "a@b.com", "user"), nil)
	assertStatus(t, resp, 200)
	assertCode(t, r, 0)
}

func TestCancelOrder_OK(t *testing.T) {
	ts, jwt := setupTestRouter()
	defer ts.Close()
	resp, r := doReq(t, "POST", ts.URL+"/api/v1/orders/1001/cancel", genToken(jwt, 1, "a@b.com", "user"), nil)
	assertStatus(t, resp, 200)
	assertCode(t, r, 0)
}

func TestCancelOrder_InvalidID(t *testing.T) {
	ts, jwt := setupTestRouter()
	defer ts.Close()
	resp, r := doReq(t, "POST", ts.URL+"/api/v1/orders/abc/cancel", genToken(jwt, 1, "a@b.com", "user"), nil)
	assertStatus(t, resp, 400)
	assertCode(t, r, 400)
}

// ============================================================
// Payments: Create / Process / Status / Refund / ByOrder (protected)
// ============================================================

func TestCreatePayment_OK(t *testing.T) {
	ts, jwt := setupTestRouter()
	defer ts.Close()
	resp, r := doReq(t, "POST", ts.URL+"/api/v1/payments", genToken(jwt, 1, "a@b.com", "user"),
		map[string]interface{}{"order_id": 1001, "amount": 29.99, "method": "wechat"})
	assertStatus(t, resp, 200)
	assertCode(t, r, 0)
}

func TestProcessPayment_OK(t *testing.T) {
	ts, jwt := setupTestRouter()
	defer ts.Close()
	resp, r := doReq(t, "POST", ts.URL+"/api/v1/payments/PAY-001/process", genToken(jwt, 1, "a@b.com", "user"), nil)
	assertStatus(t, resp, 200)
	assertCode(t, r, 0)
}

func TestGetPaymentStatus_OK(t *testing.T) {
	ts, jwt := setupTestRouter()
	defer ts.Close()
	resp, r := doReq(t, "GET", ts.URL+"/api/v1/payments/PAY-001", genToken(jwt, 1, "a@b.com", "user"), nil)
	assertStatus(t, resp, 200)
	assertCode(t, r, 0)
}

func TestRefundPayment_OK(t *testing.T) {
	ts, jwt := setupTestRouter()
	defer ts.Close()
	resp, r := doReq(t, "POST", ts.URL+"/api/v1/payments/PAY-001/refund", genToken(jwt, 1, "a@b.com", "user"),
		map[string]interface{}{"reason": "not needed"})
	assertStatus(t, resp, 200)
	assertCode(t, r, 0)
}

func TestGetPaymentByOrder_OK(t *testing.T) {
	ts, jwt := setupTestRouter()
	defer ts.Close()
	resp, r := doReq(t, "GET", ts.URL+"/api/v1/payments/order/1001", genToken(jwt, 1, "a@b.com", "user"), nil)
	assertStatus(t, resp, 200)
	assertCode(t, r, 0)
}

func TestGetPaymentByOrder_InvalidID(t *testing.T) {
	ts, jwt := setupTestRouter()
	defer ts.Close()
	resp, r := doReq(t, "GET", ts.URL+"/api/v1/payments/order/abc", genToken(jwt, 1, "a@b.com", "user"), nil)
	assertStatus(t, resp, 400)
	assertCode(t, r, 400)
}

// ============================================================
// AI: Public (summary / search / similar) + Protected (recommend / chat / taste)
// ============================================================

func TestGenerateBookSummary_OK(t *testing.T) {
	ts, _ := setupTestRouter()
	defer ts.Close()
	resp, r := doReq(t, "GET", ts.URL+"/api/v1/ai/summary/book-1", "", nil)
	assertStatus(t, resp, 200)
	assertCode(t, r, 0)
}

func TestSmartSearch_OK(t *testing.T) {
	ts, _ := setupTestRouter()
	defer ts.Close()
	resp, r := doReq(t, "POST", ts.URL+"/api/v1/ai/search", "",
		map[string]interface{}{"query": "machine learning books", "limit": 5})
	assertStatus(t, resp, 200)
	assertCode(t, r, 0)
}

func TestGetSimilarBooks_OK(t *testing.T) {
	ts, _ := setupTestRouter()
	defer ts.Close()
	resp, r := doReq(t, "GET", ts.URL+"/api/v1/ai/similar/book-1?limit=3", "", nil)
	assertStatus(t, resp, 200)
	assertCode(t, r, 0)
}

func TestGetRecommendations_OK(t *testing.T) {
	ts, jwt := setupTestRouter()
	defer ts.Close()
	resp, r := doReq(t, "POST", ts.URL+"/api/v1/ai/recommend", genToken(jwt, 1, "a@b.com", "user"),
		map[string]interface{}{"context": "I love sci-fi", "limit": 5})
	assertStatus(t, resp, 200)
	assertCode(t, r, 0)
}

func TestGetRecommendations_NoAuth(t *testing.T) {
	ts, _ := setupTestRouter()
	defer ts.Close()
	resp, _ := doReq(t, "POST", ts.URL+"/api/v1/ai/recommend", "",
		map[string]interface{}{"context": "sci-fi"})
	assertStatus(t, resp, 401)
}

func TestChatWithLibrarian_OK(t *testing.T) {
	ts, jwt := setupTestRouter()
	defer ts.Close()
	resp, r := doReq(t, "POST", ts.URL+"/api/v1/ai/chat", genToken(jwt, 1, "a@b.com", "user"),
		map[string]interface{}{"message": "recommend a book", "session_id": "sess-1"})
	assertStatus(t, resp, 200)
	assertCode(t, r, 0)
}

func TestAnalyzeReadingTaste_OK(t *testing.T) {
	ts, jwt := setupTestRouter()
	defer ts.Close()
	resp, r := doReq(t, "GET", ts.URL+"/api/v1/ai/taste", genToken(jwt, 1, "a@b.com", "user"), nil)
	assertStatus(t, resp, 200)
	assertCode(t, r, 0)
}

func TestAnalyzeReadingTaste_NoAuth(t *testing.T) {
	ts, _ := setupTestRouter()
	defer ts.Close()
	resp, _ := doReq(t, "GET", ts.URL+"/api/v1/ai/taste", "", nil)
	assertStatus(t, resp, 401)
}

// ============================================================
// Auth middleware edge cases
// ============================================================

func TestAuth_InvalidToken(t *testing.T) {
	ts, _ := setupTestRouter()
	defer ts.Close()
	resp, r := doReq(t, "GET", ts.URL+"/api/v1/user/profile", "garbage-token", nil)
	assertStatus(t, resp, 401)
	assertCode(t, r, 401)
}

func TestAuth_ExpiredToken(t *testing.T) {
	ts, _ := setupTestRouter()
	defer ts.Close()
	mgr := auth.NewJWTManager(jwtSecret, -1)
	token, _ := mgr.GenerateToken(1, "a@b.com", "user")
	resp, r := doReq(t, "GET", ts.URL+"/api/v1/user/profile", token, nil)
	assertStatus(t, resp, 401)
	assertCode(t, r, 401)
}

// ============================================================
// Service error propagation
// ============================================================

func TestServiceError_Propagation(t *testing.T) {
	jwtMgr := auth.NewJWTManager(jwtSecret, 72)
	h := &handler.Handlers{
		User: &mockUserService{
			getProfileFn: func(_ context.Context, _ *userPb.GetProfileRequest, _ ...client.CallOption) (*userPb.UserProfile, error) {
				return nil, fmt.Errorf("connection refused")
			},
		},
		Book:      &mockBookService{},
		Store:     &mockStoreService{},
		Inventory: &mockInventoryService{},
		Cart:      &mockCartService{},
		Order:     &mockOrderService{},
		Payment:   &mockPaymentService{},
		AI:        &mockAIService{},
	}
	r := router.SetupRouter(h, jwtMgr)
	ts := httptest.NewServer(r)
	defer ts.Close()

	token := genToken(jwtMgr, 1, "a@b.com", "user")
	resp, result := doReq(t, "GET", ts.URL+"/api/v1/user/profile", token, nil)
	assertStatus(t, resp, 500)
	assertCode(t, result, 500)
}

// ============================================================
// Auth: Send verification code
// ============================================================

func TestSendVerificationCode_OK(t *testing.T) {
	ts, _ := setupTestRouter()
	defer ts.Close()
	resp, r := doReq(t, "POST", ts.URL+"/api/v1/auth/send-code", "",
		map[string]string{"email": "user@example.com"})
	assertStatus(t, resp, 200)
	assertCode(t, r, 0)
}

func TestSendVerificationCode_EmptyBody(t *testing.T) {
	ts, _ := setupTestRouter()
	defer ts.Close()
	resp, r := doReq(t, "POST", ts.URL+"/api/v1/auth/send-code", "", nil)
	assertStatus(t, resp, 400)
	assertCode(t, r, 400)
}

// ============================================================
// Upload: storage not configured (unit default Handlers.Storage == nil)
// ============================================================

func TestUpload_NoAuth(t *testing.T) {
	ts, _ := setupTestRouter()
	defer ts.Close()
	resp, r := doReq(t, "POST", ts.URL+"/api/v1/upload", "", nil)
	assertStatus(t, resp, 401)
	assertCode(t, r, 401)
}

func TestUpload_StorageNotConfigured(t *testing.T) {
	ts, jwt := setupTestRouter()
	defer ts.Close()
	resp, r := doReq(t, "POST", ts.URL+"/api/v1/upload", genToken(jwt, 1, "a@b.com", "user"), nil)
	assertStatus(t, resp, 500)
	assertCode(t, r, 500)
}

// ============================================================
// AI: validation & stream error path (mock StreamChat returns error)
// ============================================================

func TestChatWithLibrarian_NoAuth(t *testing.T) {
	ts, _ := setupTestRouter()
	defer ts.Close()
	resp, r := doReq(t, "POST", ts.URL+"/api/v1/ai/chat", "",
		map[string]interface{}{"message": "hi", "session_id": "s1"})
	assertStatus(t, resp, 401)
	assertCode(t, r, 401)
}

func TestChatWithLibrarian_EmptyBody(t *testing.T) {
	ts, jwt := setupTestRouter()
	defer ts.Close()
	resp, r := doReq(t, "POST", ts.URL+"/api/v1/ai/chat", genToken(jwt, 1, "a@b.com", "user"), nil)
	assertStatus(t, resp, 400)
	assertCode(t, r, 400)
}

func TestStreamChat_ServiceUnavailable(t *testing.T) {
	ts, jwt := setupTestRouter()
	defer ts.Close()
	resp, r := doReq(t, "POST", ts.URL+"/api/v1/ai/chat/stream", genToken(jwt, 1, "a@b.com", "user"),
		map[string]interface{}{"message": "hi", "session_id": "s1"})
	assertStatus(t, resp, 500)
	assertCode(t, r, 500)
}

func TestSmartSearch_EmptyBody(t *testing.T) {
	ts, _ := setupTestRouter()
	defer ts.Close()
	resp, r := doReq(t, "POST", ts.URL+"/api/v1/ai/search", "", nil)
	assertStatus(t, resp, 400)
	assertCode(t, r, 400)
}

func TestGetRecommendations_EmptyBody(t *testing.T) {
	ts, jwt := setupTestRouter()
	defer ts.Close()
	resp, r := doReq(t, "POST", ts.URL+"/api/v1/ai/recommend", genToken(jwt, 1, "a@b.com", "user"), nil)
	assertStatus(t, resp, 400)
	assertCode(t, r, 400)
}

// ============================================================
// CORS preflight
// ============================================================

func TestCORS_OPTIONS_Preflight(t *testing.T) {
	ts, _ := setupTestRouter()
	defer ts.Close()
	req, err := http.NewRequest(http.MethodOptions, ts.URL+"/api/v1/cart", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "GET")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("OPTIONS status: want %d, got %d", http.StatusNoContent, resp.StatusCode)
	}
}

// ============================================================
// Orders / payments: auth required
// ============================================================

func TestCreateOrder_NoAuth(t *testing.T) {
	ts, _ := setupTestRouter()
	defer ts.Close()
	resp, _ := doReq(t, "POST", ts.URL+"/api/v1/orders", "", map[string]interface{}{"store_id": 1})
	assertStatus(t, resp, 401)
}

func TestCreatePayment_NoAuth(t *testing.T) {
	ts, _ := setupTestRouter()
	defer ts.Close()
	resp, _ := doReq(t, "POST", ts.URL+"/api/v1/payments", "", map[string]interface{}{"order_id": 1})
	assertStatus(t, resp, 401)
}

// TestProtectedRoutes_AllRequireAuth hits every JWT-protected route once without a token.
// It complements individual OK/error tests and catches new routes accidentally left open.
func TestProtectedRoutes_AllRequireAuth(t *testing.T) {
	ts, _ := setupTestRouter()
	defer ts.Close()
	base := ts.URL
	cases := []struct {
		method, path string
		body         interface{}
	}{
		{"GET", "/api/v1/user/profile", nil},
		{"PUT", "/api/v1/user/profile", map[string]string{"name": "x", "phone": "1"}},
		{"GET", "/api/v1/user/preferences", nil},
		{"PUT", "/api/v1/user/preferences", map[string]interface{}{"favorite_categories": []string{"fiction"}}},
		{"GET", "/api/v1/user/addresses", nil},
		{"POST", "/api/v1/user/addresses", map[string]interface{}{
			"name": "n", "phone": "13800138000", "province": "p", "city": "c", "district": "d", "detail": "addr",
		}},
		{"POST", "/api/v1/upload", nil},
		{"POST", "/api/v1/books", map[string]interface{}{
			"title": "t", "author": "a", "isbn": "978-0-00-000000-0", "price": 1.0, "category": "c",
		}},
		{"POST", "/api/v1/books/upload-cover", nil},
		{"GET", "/api/v1/cart", nil},
		{"POST", "/api/v1/cart/items", map[string]interface{}{"store_id": 1, "book_id": "b1", "quantity": 1}},
		{"DELETE", "/api/v1/cart/items/item-1", nil},
		{"PUT", "/api/v1/cart/items/item-1", map[string]interface{}{"quantity": 1}},
		{"DELETE", "/api/v1/cart", nil},
		{"POST", "/api/v1/orders", map[string]interface{}{
			"store_id": 1,
			"items":    []map[string]interface{}{{"book_id": "b1", "book_title": "t", "price": 1.0, "quantity": 1}},
			"pickup_method": "self_pickup",
		}},
		{"GET", "/api/v1/orders?page=1&page_size=10", nil},
		{"GET", "/api/v1/orders/1001", nil},
		{"POST", "/api/v1/orders/1001/cancel", nil},
		{"POST", "/api/v1/payments", map[string]interface{}{"order_id": 1001, "amount": 29.99, "method": "wechat"}},
		{"POST", "/api/v1/payments/PAY-001/process", nil},
		{"GET", "/api/v1/payments/PAY-001", nil},
		{"POST", "/api/v1/payments/PAY-001/refund", map[string]string{"reason": "test"}},
		{"GET", "/api/v1/payments/order/1001", nil},
		{"POST", "/api/v1/ai/recommend", map[string]interface{}{"context": "sci-fi", "limit": 5}},
		{"POST", "/api/v1/ai/chat", map[string]interface{}{"message": "hi", "session_id": "s1"}},
		{"POST", "/api/v1/ai/chat/stream", map[string]interface{}{"message": "hi", "session_id": "s1"}},
		{"GET", "/api/v1/ai/taste", nil},
	}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			resp, _ := doReq(t, tc.method, base+tc.path, "", tc.body)
			assertStatus(t, resp, http.StatusUnauthorized)
		})
	}
}
