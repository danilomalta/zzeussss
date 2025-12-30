package main

import (
	"errors"
	"log"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// --- Configuration ---
const (
	SecretKey     = "SUPER_SECRET_KEY_CHANGE_IN_PROD" // In prod, use os.Getenv
	BCryptCost    = 14
	TokenDuration = 24 * time.Hour
)

// --- Database Models ---

type Company struct {
	gorm.Model
	Name    string `json:"name"`
	CNPJ    string `gorm:"unique" json:"cnpj"`
	Users   []User
	Clients []Client
}

type User struct {
	gorm.Model
	Email     string `gorm:"unique" json:"email"`
	Password  string `json:"-"`    // Never return password
	Role      string `json:"role"` // "admin", "staff"
	CompanyID uint   `json:"company_id"`
}

type Client struct {
	gorm.Model
	Name      string `json:"name"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
	CompanyID uint   `json:"company_id"`
}

// --- Database Connection ---

var DB *gorm.DB

func ConnectDB() {
	var err error
	DB, err = gorm.Open(sqlite.Open("titan_secure.db"), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	// AutoMigrate
	err = DB.AutoMigrate(&Company{}, &User{}, &Client{})
	if err != nil {
		log.Fatal("Failed to migrate database:", err)
	}
	log.Println("Database connected and migrated successfully.")
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
			return []byte(SecretKey), nil
		})

		if err != nil || !token.Valid {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid or expired token"})
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token claims"})
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

	// Hash Password
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
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid credentials"})
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid credentials"})
	}

	// Generate JWT
	claims := jwt.MapClaims{
		"user_id":    user.ID,
		"company_id": user.CompanyID,
		"role":       user.Role,
		"exp":        time.Now().Add(TokenDuration).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	t, err := token.SignedString([]byte(SecretKey))
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
	var clients []Client
	if err := DB.Where("company_id = ?", companyID).Find(&clients).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Could not fetch clients"})
	}
	return c.JSON(clients)
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

	// Strict CORS
	app.Use(cors.New(cors.Config{
		AllowOrigins: "http://localhost:5173",
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
		AllowMethods: "GET, POST, PUT, DELETE, OPTIONS",
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

	log.Fatal(app.Listen(":3000"))
}
