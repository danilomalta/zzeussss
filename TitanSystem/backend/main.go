package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"log"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/helmet"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/argon2"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// --- CONFIGURATION ---
var (
	JWTSecret = []byte(os.Getenv("JWT_SECRET"))
	AESKey    = []byte(os.Getenv("AES_KEY")) // Must be 32 bytes
	DB        *gorm.DB
)

func init() {
	if len(JWTSecret) == 0 {
		JWTSecret = []byte("titan_super_secret_key_change_in_production")
	}
	if len(AESKey) != 32 {
		log.Println("⚠️  AES_KEY not set or invalid length. Using dev key.")
		AESKey = []byte("12345678901234567890123456789012")
	}
}

// --- MODELS ---

type Company struct {
	gorm.Model
	Name          string `json:"name"`
	Email         string `json:"email"`
	EncryptedData []byte `json:"-"` // CNPJ/CPF Encrypted
	BusinessSize  string `json:"business_size"`
	Sector        string `json:"sector"`
	StorageType   string `json:"storage_type"`
	PlanType      string `json:"plan_type"`
	Users         []User
	Billings      []Billing
}

type User struct {
	gorm.Model
	Email     string `gorm:"uniqueIndex" json:"email"`
	Password  string `json:"-"` // Argon2 Hash
	Role      string `json:"role"`
	CompanyID uint   `json:"company_id"`
}

type Billing struct {
	gorm.Model
	CompanyID     uint      `json:"company_id"`
	Status        string    `json:"status"` // Active, Overdue
	NextDueDate   time.Time `json:"next_due_date"`
	PaymentMethod string    `json:"payment_method"`
	Amount        float64   `json:"amount"`
}

type AuditLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `json:"user_id"`
	Action    string    `json:"action"`
	IP        string    `json:"ip"`
	UserAgent string    `json:"user_agent"`
	Timestamp time.Time `json:"timestamp"`
	RiskLevel string    `json:"risk_level"`
}

// --- SECURITY UTILS ---

func Encrypt(plaintext string) (string, error) {
	block, err := aes.NewCipher(AESKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func Decrypt(cryptoText string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(cryptoText)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(AESKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

func HashPassword(password string) string {
	salt := make([]byte, 16)
	rand.Read(salt)
	hash := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)
	return hex.EncodeToString(salt) + ":" + hex.EncodeToString(hash)
}

func VerifyPassword(password, encodedHash string) bool {
	parts := split(encodedHash, ":")
	if len(parts) != 2 {
		return false
	}
	salt, _ := hex.DecodeString(parts[0])
	targetHash, _ := hex.DecodeString(parts[1])

	hash := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)
	return string(hash) == string(targetHash)
}

func split(s, sep string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(s); i++ {
		if i+len(sep) <= len(s) && s[i:i+len(sep)] == sep {
			parts = append(parts, s[start:i])
			start = i + len(sep)
			i += len(sep) - 1
		}
	}
	parts = append(parts, s[start:])
	return parts
}

// --- MIDDLEWARE ---

func AuditLogger() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		err := c.Next()

		go func() {
			userID := uint(0)
			if claims, ok := c.Locals("user").(*jwt.Token); ok {
				if mapClaims, ok := claims.Claims.(jwt.MapClaims); ok {
					if id, ok := mapClaims["user_id"].(float64); ok {
						userID = uint(id)
					}
				}
			}

			risk := "Low"
			if c.Response().StatusCode() >= 400 {
				risk = "Medium"
			}
			if c.Response().StatusCode() == 401 || c.Response().StatusCode() == 403 {
				risk = "High"
			}

			DB.Create(&AuditLog{
				UserID:    userID,
				Action:    c.Method() + " " + c.Path(),
				IP:        c.IP(),
				UserAgent: c.Get("User-Agent"),
				Timestamp: start,
				RiskLevel: risk,
			})
		}()

		return err
	}
}

// --- HANDLERS ---

type RegisterRequest struct {
	CompanyName  string `json:"companyName"`
	Email        string `json:"email"`
	Password     string `json:"password"`
	Plan         string `json:"plan"`
	CNPJ         string `json:"cnpj"`
	BusinessSize string `json:"businessSize"`
	Sector       string `json:"sector"`
	StorageType  string `json:"storageType"`
}

func Register(c *fiber.Ctx) error {
	var req RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request"})
	}

	encryptedCNPJ, err := Encrypt(req.CNPJ)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Encryption failed"})
	}

	hashedPassword := HashPassword(req.Password)

	tx := DB.Begin()

	company := Company{
		Name:          req.CompanyName,
		Email:         req.Email,
		EncryptedData: []byte(encryptedCNPJ),
		BusinessSize:  req.BusinessSize,
		Sector:        req.Sector,
		StorageType:   req.StorageType,
		PlanType:      req.Plan,
	}
	if err := tx.Create(&company).Error; err != nil {
		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Could not create company"})
	}

	user := User{
		Email:     req.Email,
		Password:  hashedPassword,
		Role:      "admin",
		CompanyID: company.ID,
	}
	if err := tx.Create(&user).Error; err != nil {
		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Could not create user"})
	}

	billing := Billing{
		CompanyID:     company.ID,
		Status:        "Active",
		NextDueDate:   time.Now().AddDate(0, 1, 0),
		PaymentMethod: "Credit Card",
		Amount:        99.00,
	}
	if err := tx.Create(&billing).Error; err != nil {
		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Could not create billing"})
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

	if !VerifyPassword(req.Password, user.Password) {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid credentials"})
	}

	claims := jwt.MapClaims{
		"user_id":    user.ID,
		"company_id": user.CompanyID,
		"role":       user.Role,
		"exp":        time.Now().Add(time.Minute * 15).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	t, err := token.SignedString(JWTSecret)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Could not login"})
	}

	c.Cookie(&fiber.Cookie{
		Name:     "jwt",
		Value:    t,
		Expires:  time.Now().Add(time.Minute * 15),
		HTTPOnly: true,
		Secure:   true,
		SameSite: "Strict",
	})

	return c.JSON(fiber.Map{
		"token": t,
		"user": fiber.Map{
			"email": user.Email,
			"role":  user.Role,
		},
	})
}

func GetBillingStatus(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"status":   "Active",
		"active":   true,
		"plan":     "Pro",
		"due_date": time.Now().AddDate(0, 0, 15),
	})
}

// --- MAIN ---

func main() {
	var err error
	driver := os.Getenv("DB_DRIVER")
	dsn := os.Getenv("DB_DSN")

	if driver == "postgres" {
		DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
			PrepareStmt: true,
			Logger:      logger.Default.LogMode(logger.Error),
		})
	} else {
		DB, err = gorm.Open(sqlite.Open("titan_secure.db?_journal_mode=WAL"), &gorm.Config{
			PrepareStmt: true,
			Logger:      logger.Default.LogMode(logger.Silent),
		})
	}

	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	DB.AutoMigrate(&Company{}, &User{}, &Billing{}, &AuditLog{})

	app := fiber.New(fiber.Config{
		AppName: "Titan System Backend",
	})

	app.Use(helmet.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins:     "http://localhost:5173",
		AllowCredentials: true,
	}))
	app.Use(AuditLogger())

	// Rate Limiter: 5 attempts per minute
	loginLimiter := limiter.New(limiter.Config{
		Max:        5,
		Expiration: 1 * time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "Too many login attempts. Try again in 1 minute.",
			})
		},
	})

	api := app.Group("/api")

	auth := api.Group("/auth")
	auth.Post("/register", Register)
	auth.Post("/login", loginLimiter, Login)

	billing := api.Group("/billing")
	billing.Get("/status", GetBillingStatus)

	log.Println("🚀 Titan Backend (Military Grade Security) running on port 3000")
	log.Fatal(app.Listen(":3000"))
}
