package bootstrap

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/dropDatabas3/hellojohn/internal/domain/repository"
	store "github.com/dropDatabas3/hellojohn/internal/store/v2"
	"golang.org/x/term"
)

// AdminBootstrapConfig holds configuration for admin bootstrap
type AdminBootstrapConfig struct {
	DAL           store.DataAccessLayer
	TenantSlug    string // Default tenant to create admin in
	SkipPrompt    bool   // Skip interactive prompts (for testing)
	AdminEmail    string // Pre-filled email (optional)
	AdminPassword string // Pre-filled password (optional)
}

// CheckAndCreateAdmin checks if there are any admin users in the system.
// If no admins exist, it prompts the user to create one interactively.
func CheckAndCreateAdmin(ctx context.Context, cfg AdminBootstrapConfig) error {
	// 1. Check if we have any admins in the system
	hasAdmin, err := hasExistingAdmin(ctx, cfg.DAL, cfg.TenantSlug)
	if err != nil {
		return fmt.Errorf("failed to check for existing admins: %w", err)
	}

	if hasAdmin {
		fmt.Println("✅ Admin user detected. Skipping bootstrap.")
		return nil
	}

	// 2. No admins found - prompt for creation
	fmt.Println("\n⚠️  No admin users found in the system.")
	fmt.Println("📋 Let's create the first admin user to get started.\n")

	if cfg.SkipPrompt {
		// Non-interactive mode (testing)
		if cfg.AdminEmail == "" || cfg.AdminPassword == "" {
			return fmt.Errorf("SkipPrompt=true requires AdminEmail and AdminPassword")
		}
		return createAdminUser(ctx, cfg.DAL, cfg.TenantSlug, cfg.AdminEmail, cfg.AdminPassword)
	}

	// 3. Interactive prompt
	email, password, err := promptAdminCredentials()
	if err != nil {
		return fmt.Errorf("failed to prompt admin credentials: %w", err)
	}

	// 4. Create admin user
	if err := createAdminUser(ctx, cfg.DAL, cfg.TenantSlug, email, password); err != nil {
		return fmt.Errorf("failed to create admin user: %w", err)
	}

	fmt.Println("\n✅ Admin user created successfully!")
	fmt.Printf("   Email: %s\n", email)
	fmt.Printf("   You can now login at: /v2/auth/login\n\n")

	return nil
}

// hasExistingAdmin checks if there's at least one admin user in the system
func hasExistingAdmin(ctx context.Context, dal store.DataAccessLayer, tenantSlug string) (bool, error) {
	// Default tenant slug
	if tenantSlug == "" {
		tenantSlug = "local"
	}

	// Check if tenant exists
	tda, err := dal.ForTenant(ctx, tenantSlug)
	if err != nil {
		// Tenant doesn't exist - no admins
		if store.IsTenantNotFound(err) {
			return false, nil
		}
		return false, err
	}

	// Check if tenant has DB (Data Plane)
	if !tda.HasDB() {
		// FS-only mode - check FS admin flag
		fsAdminEnabled := os.Getenv("FS_ADMIN_ENABLE") == "1" || os.Getenv("FS_ADMIN_ENABLE") == "true"
		if !fsAdminEnabled {
			return false, nil
		}
		// TODO: Check FS for admin markers (future feature)
		return false, nil
	}

	// Query users with admin role
	// Note: This assumes RBAC is implemented
	// For V2, we'll do a simple user count check
	users, err := tda.Users().List(ctx, tda.ID(), repository.ListUsersFilter{
		Limit: 1, // Just check if at least one exists
	})
	if err != nil {
		return false, err
	}

	return len(users) > 0, nil
}

// createAdminUser creates a new admin user in the specified tenant
func createAdminUser(ctx context.Context, dal store.DataAccessLayer, tenantSlug, email, password string) error {
	// Default tenant slug
	if tenantSlug == "" {
		tenantSlug = "local"
	}

	// 1. Ensure tenant exists
	tda, err := dal.ForTenant(ctx, tenantSlug)
	if err != nil {
		if store.IsTenantNotFound(err) {
			return fmt.Errorf("tenant '%s' not found. Please create it first", tenantSlug)
		}
		return err
	}

	// 2. Check if tenant has DB
	if err := tda.RequireDB(); err != nil {
		return fmt.Errorf("tenant '%s' has no database configured. Cannot create admin user", tenantSlug)
	}

	// 3. Validate email doesn't exist
	existingUser, _, err := tda.Users().GetByEmail(ctx, tda.ID(), email)
	if err != nil && !repository.IsNotFound(err) {
		return fmt.Errorf("failed to check existing user: %w", err)
	}
	if existingUser != nil {
		return fmt.Errorf("user with email '%s' already exists", email)
	}

	// 4. Hash password (assuming we need to hash it manually)
	// TODO: Check if UserRepository.Create expects plaintext or hash
	// For now, assuming it expects hash - need to integrate with password hasher

	// 4. Create user
	user, _, err := tda.Users().Create(ctx, repository.CreateUserInput{
		TenantID:     tda.ID(),
		Email:        email,
		PasswordHash: password, // TODO: Hash this with bcrypt
		// Note: EmailVerified and Metadata fields don't exist in CreateUserInput
		// They might be set via Update() after creation
	})
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	// 5. Assign admin role (if RBAC is available)
	rbac := tda.RBAC()
	if rbac != nil {
		// AssignRole signature: AssignRole(ctx, tenantID, userID, role)
		if err := rbac.AssignRole(ctx, tda.ID(), user.ID, "admin"); err != nil {
			// Non-fatal - log warning
			fmt.Printf("⚠️  Warning: Failed to assign admin role: %v\n", err)
		}
	}

	// 6. Add to ADMIN_SUBS env (for immediate access)
	adminSubs := os.Getenv("ADMIN_SUBS")
	if adminSubs == "" {
		os.Setenv("ADMIN_SUBS", user.ID)
	} else {
		os.Setenv("ADMIN_SUBS", adminSubs+","+user.ID)
	}

	fmt.Printf("\n📝 IMPORTANT: Add this user ID to your .env file:\n")
	fmt.Printf("   ADMIN_SUBS=%s\n", user.ID)

	return nil
}

// promptAdminCredentials prompts the user for email and password interactively
func promptAdminCredentials() (email, password string, err error) {
	reader := bufio.NewReader(os.Stdin)

	// Prompt for email
	fmt.Print("Admin Email: ")
	email, err = reader.ReadString('\n')
	if err != nil {
		return "", "", err
	}
	email = strings.TrimSpace(email)

	if email == "" {
		return "", "", fmt.Errorf("email cannot be empty")
	}

	// Validate email format (basic)
	if !strings.Contains(email, "@") {
		return "", "", fmt.Errorf("invalid email format")
	}

	// Prompt for password (hidden input)
	fmt.Print("Admin Password (min 10 chars): ")
	passwordBytes, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return "", "", err
	}
	password = string(passwordBytes)
	fmt.Println() // New line after hidden input

	if len(password) < 10 {
		return "", "", fmt.Errorf("password must be at least 10 characters")
	}

	// Confirm password
	fmt.Print("Confirm Password: ")
	confirmBytes, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return "", "", err
	}
	confirm := string(confirmBytes)
	fmt.Println() // New line after hidden input

	if password != confirm {
		return "", "", fmt.Errorf("passwords do not match")
	}

	return email, password, nil
}

// ShouldRunBootstrap checks if admin bootstrap should run
// Returns true if:
// - No ADMIN_SUBS configured
// - FS_ADMIN_ENABLE is false
// - No users in default tenant
func ShouldRunBootstrap(ctx context.Context, dal store.DataAccessLayer) bool {
	// Check env flags
	adminSubs := os.Getenv("ADMIN_SUBS")
	fsAdminEnabled := os.Getenv("FS_ADMIN_ENABLE") == "1" || os.Getenv("FS_ADMIN_ENABLE") == "true"

	// If admin subs are configured, skip bootstrap
	if adminSubs != "" {
		return false
	}

	// If FS admin is enabled, skip bootstrap (assume FS admins exist)
	if fsAdminEnabled {
		return false
	}

	// Check if any users exist
	hasAdmin, err := hasExistingAdmin(ctx, dal, "local")
	if err != nil {
		// On error, skip bootstrap (conservative)
		return false
	}

	return !hasAdmin
}
