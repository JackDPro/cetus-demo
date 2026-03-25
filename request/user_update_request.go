package request

type UserUpdateRequest struct {
	Nickname string `form:"nickname" json:"nickname"`
	Password string `binding:"omitempty,min=8,max=24" form:"password" json:"password"`
	Avatar   string `form:"avatar" json:"avatar"`
}