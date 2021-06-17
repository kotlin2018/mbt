package example

import "time"

const (
	MysqlUri = "root:root@(127.0.0.1:3306)/test?charset=utf8&parseTime=True&loc=Local"
	MysqlUri1 = "root:root@(127.0.0.1:3306)/test1?charset=utf8&parseTime=True&loc=Local"
)

//定义数据库表的模型.
//例子：Activity 活动数据,注意 注意 json的 tag就是数据库中的字段.
type Activity struct {
	Id         string    `json:"id"`
	Uuid       string    `json:"uuid"`
	Name       string    `json:"name"`
	PcLink     string    `json:"pc_link"`
	H5Link     string    `json:"h5_link"`
	Remark     string    `json:"remark"`
	Sort       int       `json:"sort"`
	Status     int       `json:"status"`
	Version    int       `json:"version"`
	CreateTime time.Time `json:"create_time"`
	DeleteFlag int       `json:"delete_flag"`
}

// 这是 biz_activity.xml 文件对应的表的模型实体,注意 json的 tag就是数据库中的字段.
type BizActivity struct {
	Id         string    `json:"id,omitempty"`
	Name       string    `json:"name"`
	PcLink     string    `json:"pc_link"`
	H5Link     string    `json:"h5_link"`
	Remark     string    `json:"remark"`
	Sort       int       `json:"sort"`
	Status     int       `json:"status"`
	Version    int       `json:"version"`
	CreateTime time.Time `json:"create_time"`
	DeleteFlag int       `json:"delete_flag"`
}

// 数据库中并不存在 base 这张表
type Base struct {
	StartTime time.Time `json:"start_time"` // 这个 start_time 等于 xml文件中 SQL语句中的参数 start_time。
	EndTime   time.Time `json:"end_time"`   // 这个 end_time 等于 xml文件中 SQL语句中的参数 end_time。
	Size      int       `json:"size"`       // 这个 size 等于 SQL语句中的 size。
	Name      string    `json:"name"`       // 这个 name 等于 SQL语句中的 name。
	Page      int       `json:"page"`       // 这个 page 等于 SQL语句中的 page。
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

// 用户角色表
// 这是 ums_role表 的实体类
type UmsRole struct {
	Id          int64     `json:"id" form:"id"`
	Name        string    `json:"name" form:"name"`               // 名称
	Description string    `json:"description" form:"description"` // 描述
	AdminCount  int32     `json:"admin_count" form:"admin_count"` // 后台用户数量
	CreateTime  time.Time `json:"create_time" form:"create_time"` // 创建时间
	Status      int32     `json:"status" form:"status"`           // 启用状态：0-&gt;禁用；1-&gt;启用
	Sort        int32     `json:"sort" form:"sort"`
}

// 可以随意封装返回数据，只要返回数据中的字段的json标签中的值等于该方法SQL语句中的返回值即可
// GetBase()的SQL语句:select id,username,password,icon from ums_admin
type BaseInfo struct {
	Id       int64  `json:"id"`       //对应SQL中的 id
	Username string `json:"username"` //对应SQL中的 username
	Password string `json:"password"` //对应SQL中的 password
	Icon     string `json:"icon"`     //对应SQL中的 icon
}