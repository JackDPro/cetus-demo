package model

import (
	"time"

	"github.com/JackDPro/cetus/model"
	"github.com/JackDPro/cetus/provider"
	"gorm.io/gorm"
)

type User struct {
	model.BaseModel
	Id        uint64    `json:"id" gorm:"primaryKey"`
	Nickname  string    `json:"nickname"`
	Username  string    `json:"username" gorm:"unique"`
	Password  string    `json:"password" binding:"required"`
	Avatar    string    `json:"avatar"`
	Status    int       `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	DeletedAt gorm.DeletedAt
}

func (m *User) ToMap() (map[string]interface{}, error) {
	data, err := m.BaseModel.ToMap(m)
	if err != nil {
		return nil, err
	}
	data["id"] = provider.Hash().Encode(m.Id)
	return data, nil
}

func (m *User) BeforeSave(_ *gorm.DB) (err error) {
	if m.Password != "" {
		m.Password, err = provider.HashMake(m.Password)
	}
	return
}
