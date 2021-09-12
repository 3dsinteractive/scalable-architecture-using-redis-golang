package main

// Member is the struct to test in persister
type Member struct {
	ID            string `json:"id" gorm:"column:id; primary_key"`
	Username      string `json:"username" gorm:"column:username"`
	RegisterOrder int    `json:"register_order" gorm:"column:register_order"`
	IsActive      int    `json:"is_active" gorm:"column:is_active"`
}

// TableName specify table name
func (*Member) TableName() string {
	return "members"
}
