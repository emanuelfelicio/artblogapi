package auth

type RegisterRequest struct {
	// Username accepts only lowercase alphanumeric characters, dots, and underscores.
	Username string `json:"username" binding:"required,min=3,max=30,username_chars"`
	Email    string `json:"email" binding:"required,email,lowercase,max=255"`
	// Password max length of 72 bytes is due to bcrypt's maximum password length limitation.
	Password string `json:"password" binding:"required,min=8,maxbytes=72,containsany=!@#$%^&*"`
}

type RegisterResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int64  `json:"expires_in"`
}

type LoginRequest struct {
	// Credential accepts email or username.
	Credential string `json:"credential" binding:"required,min=3,max=255"`
	Password   string `json:"password" binding:"required,min=8"`
}

type LoginResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int64  `json:"expires_in"`
}
