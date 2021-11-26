package example

import "time"

// 这是 biz_activity.xml 文件对应的表的模型实体,注意 db 的 tag就是数据库中的字段.
type BizActivity struct {
	Id         string    `json:"id"`
	Name       string    `json:"name"`
	PcLink     string    `json:"pc_link"`
	Sort       int       `json:"sort"`
	Status     int       `json:"status"`
	Version    int       `json:"version"`
	CreateTime time.Time `json:"create_time"`
	DeleteFlag int       `json:"delete_flag"`
	Salary     float64   `json:"salary"`
}

//type BizActivity struct {
//	Id         string
//	Name       string
//	PcLink     string
//	Sort       int
//	Status     int
//	Version    int
//	CreateTime time.Time
//	DeleteFlag int
//	Salary     float64
//}

type BizActivity2 struct {
	Id         string  `json:"id"`
	Name       string  `json:"name"`
	PcLink     string  `json:"pc_link"`
	Sort       int     `json:"sort"`
	Status     int     `json:"status"`
	Version    int     `json:"version"`
	CreateTime string  `json:"create_time"`
	DeleteFlag int     `json:"delete_flag"`
	Salary     float64 `json:"salary"`
}

// 请求参数结构体
type Base struct {
	StartTime time.Time `json:"start_time"` // 这个 start_time 等于 xml文件中 SQL语句中的参数 start_time。
	EndTime   time.Time `json:"end_time"`   // 这个 end_time 等于 xml文件中 SQL语句中的参数 end_time。
	Name      string    `json:"name"`       // 这个 name 等于 SQL语句中的 name。
	Page      int       `json:"page"`       // 这个 page 等于 SQL语句中的 page。
	Size      int       `json:"size"`       // 这个 size 等于 SQL语句中的 size。
}

type Base2 struct {
	StartTime time.Time // 这个 start_time 等于 xml文件中 SQL语句中的参数 start_time。
	EndTime   time.Time // 这个 end_time 等于 xml文件中 SQL语句中的参数 end_time。
	Name      string    // 这个 name 等于 SQL语句中的 name。
	Page      int       // 这个 page 等于 SQL语句中的 page。
	Size      int       // 这个 size 等于 SQL语句中的 size。
}

type Page struct {
	Page      int       `json:"page"`       // 这个 page 等于 SQL语句中的 page。
	Size      int       `json:"size"`       // 这个 size 等于 SQL语句中的 size。
}

type Choose struct {
	Name string `json:"name"`
	PcLink string `json:"pc_link"`
}

// 结构体中的这个json tag 与 数据库表中的字段一一对应。
type UmsAdmin struct {
	Id         int64     `json:"id" db:"id"`
	ParentId   int64     `json:"parent_id" db:"parent_id"`
	Username   string    `json:"username" db:"username"`
	Password   string    `json:"password" db:"password"`
	Icon       string    `json:"icon" db:"icon"`               // 头像
	Email      string    `json:"email" db:"email"`             // 邮箱
	NickName   string    `json:"nick_name" db:"nick_name"`     // 昵称
	Note       string    `json:"note" db:"note"`               // 备注信息
	CreateTime time.Time `json:"create_time" db:"create_time"` // 创建时间
	LoginTime  time.Time `json:"login_time" db:"login_time"`   // 最后登录时间
	Status     int32     `json:"status" db:"status"`           // 帐号启用状态：0-&gt;禁用；1-&gt;启用
}

type UmsAdmin2 struct {
	Id         int64     `json:"id" db:"id"`
	ParentId   int64     `json:"parent_id" db:"parent_id"`
	Username   string    `json:"username" db:"username"`
	Password   string    `json:"password" db:"password"`
	Icon       string    `json:"icon" db:"icon"`               // 头像
	Email      string    `json:"email" db:"email"`             // 邮箱
	NickName   string    `json:"nick_name" db:"nick_name"`     // 昵称
	Note       string    `json:"note" db:"note"`               // 备注信息
	CreateTime time.Time `json:"create_time" db:"create_time"` // 创建时间
	LoginTime  time.Time `json:"login_time" db:"login_time"`   // 最后登录时间
	Status     int32     `json:"status" db:"status"`           // 帐号启用状态：0-&gt;禁用；1-&gt;启用
	UmsRole2
}

type UmsRole2 struct {
	RId         int64     `json:"r_id" form:"id" db:"r_id"`
	Pid         int       `json:"pid" form:"pid" db:"pid"`
	Name        string    `json:"name" form:"name" db:"name"`                        // 名称
	Description string    `json:"description" form:"description" db:"description"`   // 描述
	AdminCount  int32     `json:"admin_count" form:"admin_count" db:"admin_count"`   // 后台用户数量
	RCreateTime time.Time `json:"r_create_time" form:"create_time" db:"r_create_time"` // 创建时间
	RStatus     int32     `json:"r_status" form:"status" db:"r_status"`                // 启用状态：0-&gt;禁用；1-&gt;启用
	Sort        int32     `json:"sort" form:"sort" db:"sort"`
}

type UmsAdmin3 struct {
	UmsAdmin []UmsAdmin
	UmsRole2 []UmsRole2
}

// 用户角色表
// 这是 ums_role表 的实体类
type UmsRole struct {
	Id          int64     `json:"id" form:"id" db:"id"`
	Pid         int       `json:"pid" form:"pid" db:"pid"`
	Name        string    `json:"name" form:"name" db:"name"`                      // 名称
	Description string    `json:"description" form:"description" db:"description"` // 描述
	AdminCount  int32     `json:"admin_count" form:"admin_count" db:"admin_count"` // 后台用户数量
	CreateTime  time.Time `json:"create_time" form:"create_time" db:"create_time"` // 创建时间
	Status      int32     `json:"status" form:"status" db:"status"`                // 启用状态：0-&gt;禁用；1-&gt;启用
	Sort        int32     `json:"sort" form:"sort" db:"sort"`
}

// 可以随意封装返回数据，只要返回数据中的字段的json标签中的值等于该方法SQL语句中的返回值即可
// GetBase()的SQL语句:select id,username,password,icon from ums_admin
type BaseInfo struct {
	Id       int64  `json:"id" db:"id"`             //对应SQL中的 id
	Username string `json:"username" db:"username"` //对应SQL中的 username
	Password string `json:"password" db:"password"` //对应SQL中的 password
	Icon     string `json:"icon" db:"icon"`         //对应SQL中的 icon
}

type Activity2 struct {
	Id   string    `json:"id" db:"id"`
	Info
}

type Info struct {
	Name       string  `json:"name" db:"name"`
	PcLink     string  `json:"pc_link" db:"pc_link"`
	Sort       int     `json:"sort" db:"sort"`
	Status     int     `json:"status" db:"status"`
	Version    int     `json:"version" db:"version"`
	CreateTime string  `json:"create_time" db:"create_time"`
	DeleteFlag int     `json:"delete_flag" db:"delete_flag"`
	Salary     float64 `json:"salary" db:"salary"`
}


