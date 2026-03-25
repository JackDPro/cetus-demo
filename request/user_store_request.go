package request

type UserStoreRequest struct {
	Username string `binding:"required" form:"username" json:"username"`
	Nickname string `binding:"required" form:"nickname" json:"nickname"`
	Password string `binding:"required,min=8,max=24" form:"password" json:"password"`
}
