package request

type AuthPasswordRequest struct {
	Username string `binding:"required" form:"username" json:"username"`
	Password string `binding:"required" form:"password" json:"password"`
}
