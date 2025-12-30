package main

import (
	"errors"
	"log"
	"os"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/helmet"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// --- Configuration ---
const (
	// In production, these MUST be loaded from environment variables
	DefaultSecretKey = "SUPER_SECRET_KEY_CHANGE_IN_PROD_IMMEDIATELY"
	BCryptCost       = 12 // Increased cost for security
	TokenDuration    = 15 * time.Minute
	RefreshThreshold = 5 * time.Minute // Refresh if less than this time remains
)

func getSecretKey() []byte {
	key := os.Getenv("JWT_SECRET")
	if key == "" {
		return []byte(DefaultSecretKey)
	}
	return []byte(key)
}

// --- Database Models ---

type Company struct {
	gorm.Model
	Name    string `json:"name"`
	CNPJ    string `gorm:"uniqueIndex" json:"cnpj"` // Indexed for performance
	Users   []User
	Clients []Client
}

type User struct {
	gorm.Model
	Email     string `gorm:"uniqueIndex" json:"email"` // Indexed for performance
	Password  string `json:"-"`                        // Never return password
	Role      string `json:"role"`                     // "admin", "staff"
	CompanyID uint   `json:"company_id"`
}

type Client struct {
	gorm.Model
	Name      string `json:"name"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
	CompanyID uint   `json:"company_id" gorm:"index"` // Indexed for filtering
}

// --- Database Connection ---

var DB *gorm.DB

func ConnectDB() {
	var err error
	// Use Prepared Statements for security (SQL Injection protection) and performance
	// Enable WAL Mode for better concurrency and performance
	DB, err = gorm.Open(sqlite.Open("titan_secure.db?_journal_mode=WAL"), &gorm.Config{
		PrepareStmt: true,
		Logger:      logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	// AutoMigrate
	err = DB.AutoMigrate(&Company{}, &User{}, &Client{})
	if err != nil {
		log.Fatal("Failed to migrate database:", err)
	}
	log.Println("Database connected, migrated, and hardened successfully (WAL Mode Enabled).")
}

// --- Helper: Generate Token ---
func generateToken(user User) (string, error) {
	claims := jwt.MapClaims{
		"user_id":    user.ID,
		"company_id": user.CompanyID,
		"role":       user.Role,
		"exp":        time.Now().Add(TokenDuration).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(getSecretKey())
}

// --- Middleware ---

func Protected() fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Missing Authorization header"})
		}

		tokenString := strings.Replace(authHeader, "Bearer ", "", 1)
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, errors.New("unexpected signing method")
			}
			return getSecretKey(), nil
		})

		if err != nil || !token.Valid {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid or expired token"})
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token claims"})
		}

		// --- Sliding Window Logic ---
		if exp, ok := claims["exp"].(float64); ok {
			expirationTime := time.Unix(int64(exp), 0)
			timeRemaining := time.Until(expirationTime)

			if timeRemaining < RefreshThreshold {
				// Token is expiring soon, issue a new one
				userID := uint(claims["user_id"].(float64))
				var user User
				if err := DB.First(&user, userID).Error; err == nil {
					newToken, err := generateToken(user)
					if err == nil {
						// Send new token in header
						c.Set("X-New-Token", newToken)
						// Also expose this header to CORS
						c.Set("Access-Control-Expose-Headers", "X-New-Token")
					}
				}
			}
		}

		// Store user info in context for next handlers
		c.Locals("user_id", claims["user_id"])
		c.Locals("company_id", claims["company_id"])
		c.Locals("role", claims["role"])

		return c.Next()
	}
}

// --- Handlers ---

type RegisterRequest struct {
	CompanyName string `json:"company_name"`
	CNPJ        string `json:"cnpj"`
	Email       string `json:"email"`
	Password    string `json:"password"`
}

func Register(c *fiber.Ctx) error {
	var req RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	// Basic Validation
	if req.Email == "" || req.Password == "" || req.CompanyName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "All fields are required"})
	}

	// Transaction to ensure atomicity
	tx := DB.Begin()

	// Check if Company/User exists
	var existingUser User
	if err := tx.Where("email = ?", req.Email).First(&existingUser).Error; err == nil {
		tx.Rollback()
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "Email already registered"})
	}

	// Create Company
	company := Company{Name: req.CompanyName, CNPJ: req.CNPJ}
	if err := tx.Create(&company).Error; err != nil {
		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Could not create company"})
	}

	// Hash Password with High Cost
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), BCryptCost)
	if err != nil {
		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Could not hash password"})
	}

	// Create Admin User
	user := User{
		Email:     req.Email,
		Password:  string(hash),
		Role:      "admin",
		CompanyID: company.ID,
	}
	if err := tx.Create(&user).Error; err != nil {
		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Could not create user"})
	}

	tx.Commit()

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"message": "Registration successful"})
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func Login(c *fiber.Ctx) error {
	var req LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request"})
	}

	var user User
	if err := DB.Where("email = ?", req.Email).First(&user).Error; err != nil {
		// Use generic error message to prevent enumeration
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid credentials"})
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid credentials"})
	}

	// Generate JWT
	t, err := generateToken(user)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Could not generate token"})
	}

	return c.JSON(fiber.Map{
		"token": t,
		"user": fiber.Map{
			"id":    user.ID,
			"email": user.Email,
			"role":  user.Role,
		},
	})
}

func GetClients(c *fiber.Ctx) error {
	companyID := c.Locals("company_id")

	// Pagination
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 20) // Default 20 items per page
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100 // Hard limit to prevent memory exhaustion
	}
	offset := (page - 1) * limit

	var clients []Client
	var total int64

	// Count total for metadata
	DB.Model(&Client{}).Where("company_id = ?", companyID).Count(&total)

	// Fetch paginated data
	if err := DB.Where("company_id = ?", companyID).Limit(limit).Offset(offset).Find(&clients).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Could not fetch clients"})
	}

	return c.JSON(fiber.Map{
		"data":  clients,
		"page":  page,
		"limit": limit,
		"total": total,
	})
}

type CreateClientRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Phone string `json:"phone"`
}

func CreateClient(c *fiber.Ctx) error {
	var req CreateClientRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request"})
	}

	companyID := c.Locals("company_id").(float64) // JWT claims are float64 by default

	client := Client{
		Name:      req.Name,
		Email:     req.Email,
		Phone:     req.Phone,
		CompanyID: uint(companyID),
	}

	if err := DB.Create(&client).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Could not create client"})
	}

	return c.Status(fiber.StatusCreated).JSON(client)
}

// --- Main ---

func main() {
	ConnectDB()

	app := fiber.New()

	// --- Security Middleware ---
	// 1. Helmet: Secure Headers (XSS, Clickjacking, etc.)
	app.Use(helmet.New())

	// 2. Rate Limiting: Prevent Brute Force & DDoS
	// 20 requests per 30 seconds per IP (Stricter for "NASA-level" security)
	app.Use(limiter.New(limiter.Config{
		Max:        20,
		Expiration: 30 * time.Second,
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "Too many requests. Please try again later.",
			})
		},
	}))

	// 3. Strict CORS
	app.Use(cors.New(cors.Config{
		AllowOrigins:  "http://localhost:5173", // Only allow frontend origin
		AllowHeaders:  "Origin, Content-Type, Accept, Authorization",
		AllowMethods:  "GET, POST, PUT, DELETE, OPTIONS",
		ExposeHeaders: "X-New-Token", // Allow frontend to see the new token header
	}))

	api := app.Group("/api")

	// Public Routes
	auth := api.Group("/auth")
	auth.Post("/register", Register)
	auth.Post("/login", Login)

	// Protected Routes
	api.Use(Protected())
	api.Get("/clients", GetClients)
	api.Post("/clients", CreateClient)

	log.Println("Titan Backend running on port 3000")
	log.Fatal(app.Listen(":3000"))
}
