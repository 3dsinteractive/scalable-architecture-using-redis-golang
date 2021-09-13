package main

// Member is the struct to test in persister
type Member struct {
	ID            string `json:"id" gorm:"column:id; primary_key"`
	Username      string `json:"username" gorm:"column:username"`
	RegisterOrder int    `json:"register_order" gorm:"column:register_order"`
	MemberLevel   int    `json:"member_level" gorm:"column:member_level"`
	IsActive      int    `json:"is_active" gorm:"column:is_active"`
}

// TableName specify table name
func (*Member) TableName() string {
	return "members"
}

// MemberPoint is the struct to keep member point
type MemberPoint struct {
	Username string `json:"username" gorm:"column:username; primary_key"`
	Point    int    `json:"point" gorm:"column:point"`
}

// TableName specify table name
func (*MemberPoint) TableName() string {
	return "member_points"
}
