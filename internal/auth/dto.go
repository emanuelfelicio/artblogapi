package auth

type RegisterRequest struct {
	// Username accepts only lowercase alphanumeric characters, dots, and underscores.
	Username string `json:"username" binding:"required,min=3,max=30,username_chars" example:"john_doe"`
	Email    string `json:"email" binding:"required,email,lowercase,max=255" example:"john.doe@email.com"`
	// Password max length of 72 bytes is due to bcrypt's maximum password length limitation.
	Password string `json:"password" binding:"required,min=8,maxbytes=72,containsany=!@#$%^&*" example:"StrongP@ssw0rd!"`
}

type RegisterResponse struct {
	AccessToken string `json:"access_token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoiMTIzNDU2NzgtMTIzNC0xMjM0LTEyMzQtMTIzNDU2Nzg5MDEyIiwiZXhwIjoxNzE5Mjc4NjAwfQ.signature"`
	TokenType   string `json:"token_type" example:"Bearer"`
	ExpiresIn   int64  `json:"expires_in" example:"900"`
}

type LoginRequest struct {
	// Credential accepts email or username.
	Credential string `json:"credential" binding:"required,min=3,max=255" example:"john_doe"`
	Password   string `json:"password" binding:"required,min=8" example:"StrongP@ssw0rd!"`
}

type LoginResponse struct {
	AccessToken string `json:"access_token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoiMTIzNDU2NzgtMTIzNC0xMjM0LTEyMzQtMTIzNDU2Nzg5MDEyIiwiZXhwIjoxNzE5Mjc4NjAwfQ.signature"`
	TokenType   string `json:"token_type" example:"Bearer"`
	ExpiresIn   int64  `json:"expires_in" example:"900"`
}
