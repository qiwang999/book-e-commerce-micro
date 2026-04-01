package router

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/qiwang/book-e-commerce-micro/api-gateway/handler"
	"github.com/qiwang/book-e-commerce-micro/api-gateway/middleware"
	"github.com/qiwang/book-e-commerce-micro/common/auth"
	"github.com/qiwang/book-e-commerce-micro/common/util"
)

func SetupRouter(h *handler.Handlers, jwtMgr *auth.JWTManager) *gin.Engine {
	r := gin.Default()
	r.Use(corsMiddleware())
	r.Use(middleware.RateLimitMiddleware(100, 200))

	r.GET("/health", func(c *gin.Context) {
		util.Success(c, gin.H{"status": "ok"})
	})

	v1 := r.Group("/api/v1")

	authGroup := v1.Group("/auth")
	authGroup.Use(middleware.RateLimitMiddleware(10, 20))
	{
		authGroup.POST("/send-code", h.SendVerificationCodeHandler)
		authGroup.POST("/register", h.RegisterHandler)
		authGroup.POST("/login", h.LoginHandler)
	}

	// Public book routes
	books := v1.Group("/books")
	{
		books.GET("/search", h.SearchBooksHandler)
		books.GET("/categories", h.ListCategoriesHandler)
		books.GET("/:id", h.GetBookDetailHandler)
	}

	// Public store routes
	stores := v1.Group("/stores")
	{
		stores.GET("", h.ListStoresHandler)
		stores.GET("/nearest", h.GetNearestStoreHandler)
		stores.GET("/radius", h.GetStoresInRadiusHandler)
		stores.GET("/:id", h.GetStoreDetailHandler)
	}

	// Public inventory routes
	inventory := v1.Group("/inventory")
	{
		inventory.GET("/stock", h.CheckStockHandler)
		inventory.GET("/store/:store_id/books", h.GetStoreBooksHandler)
	}

	// Public AI routes (rate-limited)
	aiPublic := v1.Group("/ai")
	aiPublic.Use(middleware.RateLimitMiddleware(20, 40))
	{
		aiPublic.GET("/summary/:book_id", h.GenerateBookSummaryHandler)
		aiPublic.POST("/search", h.SmartSearchHandler)
		aiPublic.GET("/similar/:book_id", h.GetSimilarBooksHandler)
	}

	// Protected routes
	protected := v1.Group("")
	protected.Use(middleware.AuthMiddleware(jwtMgr))
	{
		// User profile
		user := protected.Group("/user")
		{
			user.GET("/profile", h.GetProfileHandler)
			user.PUT("/profile", h.UpdateProfileHandler)
			user.GET("/preferences", h.GetUserPreferencesHandler)
			user.PUT("/preferences", h.UpdateUserPreferencesHandler)
			user.GET("/addresses", h.ListAddressesHandler)
			user.POST("/addresses", h.CreateAddressHandler)
		}

		// File upload (authenticated users)
		protected.POST("/upload", h.UploadHandler)

		// Admin book management
		protectedBooks := protected.Group("/books")
		protectedBooks.Use(middleware.AdminMiddleware())
		{
			protectedBooks.POST("", h.CreateBookHandler)
			protectedBooks.POST("/upload-cover", h.UploadBookCoverHandler)
		}

		// Cart
		cart := protected.Group("/cart")
		{
			cart.GET("", h.GetCartHandler)
			cart.POST("/items", h.AddToCartHandler)
			cart.DELETE("/items/:item_id", h.RemoveFromCartHandler)
			cart.PUT("/items/:item_id", h.UpdateCartItemHandler)
			cart.DELETE("", h.ClearCartHandler)
		}

		// Orders
		orders := protected.Group("/orders")
		{
			orders.POST("", h.CreateOrderHandler)
			orders.GET("", h.ListOrdersHandler)
			orders.GET("/:id", h.GetOrderHandler)
			orders.POST("/:id/cancel", h.CancelOrderHandler)
		}

		// Payments
		payments := protected.Group("/payments")
		{
			payments.POST("", h.CreatePaymentHandler)
			payments.POST("/:payment_no/process", h.ProcessPaymentHandler)
			payments.GET("/:payment_no", h.GetPaymentStatusHandler)
			payments.POST("/:payment_no/refund", h.RefundPaymentHandler)
			payments.GET("/order/:order_id", h.GetPaymentByOrderHandler)
		}

		// AI (protected, rate-limited)
		aiProtected := protected.Group("/ai")
		aiProtected.Use(middleware.RateLimitMiddleware(20, 40))
		{
			aiProtected.POST("/recommend", h.GetRecommendationsHandler)
			aiProtected.POST("/chat", h.ChatWithLibrarianHandler)
			aiProtected.POST("/chat/stream", h.StreamChatHandler)
			aiProtected.GET("/taste", h.AnalyzeReadingTasteHandler)
		}
	}

	return r
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			c.Header("Access-Control-Allow-Origin", origin)
		}
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization")
		c.Header("Access-Control-Expose-Headers", "Content-Length, Content-Type")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Max-Age", "43200")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
