package example

import (
	"bufio"
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/jmoiron/sqlx"
	"github.com/kotlin2018/mbt"
	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	_ "github.com/lib/pq"
	"github.com/spf13/viper"
	_ "gorm.io/driver/sqlserver"
	//_ "github.com/go-sql-driver/mysql"
	_ "gorm.io/driver/mysql"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// 我在 v0.3.2 的基础上新增了一些功能，建议将次新功能并入到 v0.3.2 版本
var (
	en       *mbt.Session
	activity = ActivityDao{} // 这就是操作一张表
	admin    = AdminDao{}
	role     = RoleDao{}
	cfg      Config
)

// 全局总配置文件的配置结构体
type Config struct {
	Database  *mbt.Database //第一个数据库 test 的配置项
	Database2 *mbt.Database //第二个数据库 test1 的配置项
}

func init(){
	v := viper.New()
	v.SetConfigFile("./config.toml")
	err := v.ReadInConfig()
	if err !=nil {
		log.Println(err)
		panic(fmt.Errorf("Fatal error config file : %s \n",err))
	}
	v.WatchConfig()
	v.OnConfigChange(func(in fsnotify.Event) {
		log.Println("config file changed:",in.Name)
		if err = v.Unmarshal(&cfg);err !=nil {
			log.Println(err)
		}
	})

	if err = v.Unmarshal(&cfg);err !=nil {
		log.Println(err)
	}
}

func Test(t *testing.T){
	fmt.Println(cfg.Database.Pkg)
	fmt.Println(cfg.Database.DriverName)
	fmt.Println(cfg.Database.DSN)
	fmt.Println(cfg.Database.MaxOpenConn)
	fmt.Println(cfg.Database.MaxIdleConn)
	fmt.Println(cfg.Database.ConnMaxLifetime)
	fmt.Println(cfg.Database.ConnMaxIdleTime)

	fmt.Println(cfg.Database.Logger.PrintSql)
	fmt.Println(cfg.Database.Logger.PrintXml)
	fmt.Println(cfg.Database.Logger.Path)
	fmt.Println(cfg.Database.Logger.LinkName)
	fmt.Println(cfg.Database.Logger.Interval)
	fmt.Println(cfg.Database.Logger.MaxAge)
	fmt.Println(cfg.Database.Logger.Count)

	fmt.Println(cfg.Database2.Pkg)
	fmt.Println(cfg.Database2.DriverName)
	fmt.Println(cfg.Database2.DSN)
	fmt.Println(cfg.Database2.MaxOpenConn)
	fmt.Println(cfg.Database2.MaxIdleConn)
	fmt.Println(cfg.Database2.ConnMaxLifetime)
	fmt.Println(cfg.Database2.ConnMaxIdleTime)

	fmt.Println(cfg.Database2.Logger.PrintSql)
	fmt.Println(cfg.Database2.Logger.PrintXml)
	fmt.Println(cfg.Database2.Logger.Path)
	fmt.Println(cfg.Database2.Logger.LinkName)
	fmt.Println(cfg.Database2.Logger.Interval)
	fmt.Println(cfg.Database2.Logger.MaxAge)
	fmt.Println(cfg.Database2.Logger.Count)
}
func initLogger (path string,maxAge,interval int)io.Writer{
	/* 日志轮转相关函数
	 WithLinkName() 	 为最新的日志建立软连接
	 WithRotationTime()  设置日志分割的时间，隔多久分割一次
	 WithMaxAge() 和 WithRotationCount() 二者只能设置一个
	 WithMaxAge()  		 设置文件清理前的最长保存时间
	 WithRotationCount() 设置文件清理前最多保存的个数
	*/

	// 下面配置日志每隔 interval 秒轮转一个新文件，保留最近 maxAge 秒的日志文件，多余的自动清理掉。
	writer, _ := rotatelogs.New(
		path+".%Y%m%d%H%M",
		rotatelogs.WithLinkName(path),
		rotatelogs.WithMaxAge(time.Duration(maxAge)*time.Hour),
		rotatelogs.WithRotationTime(time.Duration(interval)*time.Hour),
	)
	return writer
}

// 操作 MysqlUri 对应的 test 数据库
// 注意: test 数据库中只有 biz_activity 这张表
func init(){
	//第一步: 创建一个 engine对象,一个engine对象操作一个SQL数据库。
	engine := mbt.New(cfg.Database)
	en = engine

	//第二步: 可选,生产环境下建议使用此功能
	engine.SetOutPut(initLogger(cfg.Database.Logger.Path, cfg.Database.Logger.MaxAge, cfg.Database.Logger.Interval))

	//第三步: 启动服务(如果只单纯测试项目中的SQL语句是否正确,则这一步可以先注释掉)
	engine.Run(&activity,&admin,&role)
	//engine.Run(&ActivityDao{}) // 这种写法是错误的!!!
}


// 对数据库表所有的CRUD操作都在这个结构体里面
// 原则上来说 一个 xxxDao结构体只操作一张表,实际您也可以 一个 xxxDao 结构体操作多张表。
type ActivityDao struct {
	BizActivity
	InsertTemplate      func(arg BizActivity) int64
	InsertTemplateBatch func(args []BizActivity) int64
	UpdateTemplate      func(arg BizActivity) int64
	//SelectTemplate      func(arg Activity) []Activity
	SelectTemplate      func(name, pc_link string) []BizActivity `arg:"name,pc_link"`
	SelectCountTemplate func(name string) int64                  `arg:"name"`
	DeleteTemplate      func(name string) int64                  `arg:"name"`
	SelectTpl           func(name string) BizActivity            `arg:"name"`
	SelectTpl2          func(name string) Activity2              `arg:"name"`

	GetId   func(projectId, repositoryId int64)[]int  `arg:"project_id,repository_id"`
	GetName func() string
	GetIdsByName func(name string)[]string `arg:"name"`

	GetCreatedAt func()time.Time

	SelectByIds     func(ids []string) []BizActivity       `arg:"ids"`
	SelectByIds2    func(ids []string) BizActivity         `arg:"ids"`
	SelectByIdMaps  func(ids map[int]string) []BizActivity `arg:"ids"`
	SelectAll       func() []Activity2
	SelectById      func(id string) BizActivity                `arg:"id"`
	SelectOneColumn func(id string) time.Time                  `arg:"id"`
	GetSalary       func(startTime, endTime time.Time) float64 `arg:"start_time,end_time"`

	SelectByCondition  func(base Base) []BizActivity
	SelectByCondition2 func(base Base2) []BizActivity
	SelectByName       func(name, pcLink string) BizActivity `arg:"name,pc_link"`
	//SelectByName       func(name, pcLink string) BizActivity3 `arg:"name,pc_link"`
	UpdateById       func(arg BizActivity) int64
	Insert           func(arg BizActivity) int64
	Insert2          func(arg BizActivity2) int64
	InsertCollection func(arg []BizActivity) int64
	CountByCondition func(name string, startTime time.Time, endTime time.Time) int `arg:"name,start_time,end_time"`
	RemoveById       func(id string) int64                                         `arg:"id"`
	DeleteById       func(id int) int64                                            `arg:"id"`
	DeleteByIds      func(arg []int) int                                           `arg:"ids"`

	Choose      func(deleteFlag int) []BizActivity `arg:"delete_flag"`
	Choose2     func(arg Choose) []BizActivity2
	SelectLinks func(column string) []BizActivity `arg:"column"`
}

func TestDeleteByIds(t *testing.T){
	ids := []int{0,1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,221,222,223,100,1000,18,337,449,89,180}
	res := activity.DeleteByIds(ids)
	if res >=0 {
		fmt.Println(res)
	}
}

func TestGetId(t *testing.T){
	//ids := activity.GetId(2,1)
	//for _,id := range ids {
	//	fmt.Println(id)
	//}
	ids2 := activity.GetIdsByName("刺客")
	fmt.Println(ids2)

	//fmt.Println(activity.GetName())
	//fmt.Println(activity.GetCreatedAt())
}

//插入
func TestInset(t *testing.T) {
	activity.Insert(BizActivity{Id: "1", Name: "刺客", PcLink: "韩信", CreateTime: time.Now(), DeleteFlag: 1})
	//activity.Insert2(BizActivity2{Id: "1", Name: "刺客", PcLink: "韩信", CreateTime: time.Now().String(), DeleteFlag: 1})


	//activity.Insert(BizActivity{Id: "2", Name: "刺客", PcLink: "孙悟空", CreateTime: time.Now(), DeleteFlag: 1})
	//activity.Insert(BizActivity{Id: "3", Name: "刺客", PcLink: "兰陵王", CreateTime: time.Now(), DeleteFlag: 1})
	//activity.Insert(BizActivity{Id: "4", Name: "法师", PcLink: "妲己", CreateTime: time.Now(), DeleteFlag: 1})
	//activity.Insert(BizActivity{Id: "5", Name: "法师", PcLink: "安琪拉", CreateTime: time.Now(), DeleteFlag: 1})
	//activity.Insert(BizActivity{Id: "6", Name: "战士", PcLink: "夏侯敦", CreateTime: time.Now(), DeleteFlag: 1})
	//activity.Insert(BizActivity{Id: "7", Name: "战士", PcLink: "凯", CreateTime: time.Now(), DeleteFlag: 1})
	//activity.Insert(BizActivity{Id: "8", Name: "射手", PcLink: "鲁班", CreateTime: time.Now(), DeleteFlag: 1})
	//activity.Insert(BizActivity{Id: "9", Name: "射手", PcLink: "虞姬", CreateTime: time.Now(), DeleteFlag: 1})
	//activity.Insert(BizActivity{Id: "10", Name: "射手", PcLink: "后裔", CreateTime: time.Now(), DeleteFlag: 1})
	//activity.Insert(BizActivity{Id: "11", Name: "辅助", PcLink: "太乙真人", CreateTime: time.Now(), DeleteFlag: 1})
}

func TestInsertCollection(t *testing.T){
	a1 := BizActivity{Id: "1", Name: "刺客", PcLink: "韩信", CreateTime: time.Now(), Sort:1,Status:1,DeleteFlag: 1,Salary: 10.09}
	a2 := BizActivity{Id: "2", Name: "刺客", PcLink: "孙悟空", CreateTime: time.Now(), Sort:1,Status:1,DeleteFlag: 1,Salary: 10.09}
	a3 := BizActivity{Id: "3", Name: "刺客", PcLink: "兰陵王", CreateTime: time.Now(), Sort:1,Status:1,DeleteFlag: 1,Salary: 10.09}
	a4 := BizActivity{Id: "4", Name: "法师", PcLink: "妲己", CreateTime: time.Now(), Sort:1,Status:1,DeleteFlag: 1,Salary: 10.08}
	a5 := BizActivity{Id: "5", Name: "法师", PcLink: "安琪拉", CreateTime: time.Now(), Sort:1,Status:1,DeleteFlag: 1,Salary: 10.08}
	a6 := BizActivity{Id: "6", Name: "战士", PcLink: "夏侯敦", CreateTime: time.Now(), Sort:1,Status:1,DeleteFlag: 1,Salary: 10.07}
	a7 := BizActivity{Id: "7", Name: "战士", PcLink: "凯", CreateTime: time.Now(), Sort:1,Status:1,DeleteFlag: 1,Salary: 10.06}
	a8 := BizActivity{Id: "8", Name: "射手", PcLink: "鲁班", CreateTime: time.Now(), Sort:1,Status:1,DeleteFlag: 1,Salary: 10.05}
	a9 := BizActivity{Id: "9", Name: "射手", PcLink: "虞姬", CreateTime: time.Now(), Sort:1,Status:1,DeleteFlag: 1,Salary: 10.05}
	a10 := BizActivity{Id: "10", Name: "辅助", PcLink: "太乙真人", CreateTime: time.Now(), Sort:1,Status:1,DeleteFlag: 1,Salary: 10.03}
	array := make([]BizActivity,0)
	array = append(array,a1,a2,a3,a4,a5,a6,a7,a8,a9,a10)
	res := activity.InsertCollection(array)
	if res >=0 {
		fmt.Println(res)
	}
}

func TestGetSalary(t *testing.T){
	now := time.Now()
	start := now.Add(-21*time.Hour)
	end := now.Add(1 *time.Hour)
	salary := activity.GetSalary(start,end)
	fmt.Println(salary)
}

func TestSelectColumn(t *testing.T){
	column := activity.SelectOneColumn("1")
	fmt.Println(column)
}

// 通过传入不同的参数来复用同一个方法,sql语句中 where 条件之间用 or来连接!!!
func TestChoose2(t *testing.T){
	res := activity.Choose2(Choose{Name: "刺客"})
	for _, item := range res {
		fmt.Println(item)
	}
	res2 := activity.Choose2(Choose{PcLink: "鲁班"})
	fmt.Println(res2)
}

// 返回值的结构体字段类型可以都是 string 类型
func TestSelectByName(t *testing.T) {
	res := activity.SelectByName("刺客", "韩信")
	fmt.Println(res)
}

//查询 通过传入不同的参数来复用同一个方法,sql语句中 where 条件之间用 or来连接!!!
// 注释掉:biz_activity.xml文件中 <resultMap>标签中的 <result column="delete_flag">这一项,则 delete_flag = 0的记录也会被查询出来!
func TestSelectTemplate(t *testing.T) {
	//result := activity.SelectTemplate(Activity{Name: "刺客", PcLink: "韩信"})
	result := activity.SelectTemplate("刺客","韩信")
	fmt.Println("result=", result) // [{1  刺客    1 1 0 0001-01-01 00:00:00 +0000 UTC 0 10.09}]
	// [{1  刺客 韩信   1 1 0 2021-11-14 09:19:13 +0800 CST 1 10.09}]

	// {"sort":1,"status":1,"create_time":"2021-11-14T09:19:13+08:00","delete_flag":1,"salary":10.09,"id":"1","pc_link":"韩信","name":"刺客"
}

func TestSelectTpl(t *testing.T){
	//tpl := activity.SelectTpl("辅助")
	//fmt.Println(tpl)

	res2 := activity.SelectTpl2("辅助")
	fmt.Println(res2)
}

//联合查询,分别从 test、test1数据库中查询出记录,此功能适用于分库分表。
func TestSelect(t *testing.T) {
	//base2 := Base2{
	//	Name: "刺客",
	//	Page: 3,
	//	Size: 0,
	//	StartTime: time.Now().Add(-300*time.Hour),
	//	EndTime: time.Now().Add(5*time.Hour),
	//}
	//result2 := activity.SelectByCondition2(base2)
	//fmt.Println("num=", len(result2))
	//for _,item := range result2 {
	//	fmt.Println("result=", item)
	//}

	base := Base{
		Name: "刺客",
		//Page: 3,
		//Size: 0,

		StartTime: time.Now().Add(-45*time.Hour),
		EndTime: time.Now().Add(5*time.Hour),
	}
	result := activity.SelectByCondition(base)
	fmt.Println("num=", len(result))
	for _,item := range result {
		//timeObj, _ := time.Parse(time.RFC3339, item.CreateTime.String())
		//timeFormat := timeObj.Format("2006-01-02 15:04:05")
		//fmt.Println("result=", timeFormat)
		fmt.Println("result=", item.CreateTime)
	}



	//u, err := admin.ListWithChildren()
	//if err ==nil {
	//	fmt.Println(u[0])
	//	fmt.Println(u[1])
	//	fmt.Println(u[2])
	//	fmt.Println(u[3])
	//	fmt.Println("num =",len(u))
	//}
	//
	//item, err := role.GetItem(1)
	//if err ==nil {
	//	fmt.Println(item)
	//}
}

func TestGoroutine(t *testing.T){
	wg := sync.WaitGroup{}
	for i := 0 ; i<100;i++ {
		wg.Add(1)
		go func(n int) {
			res := activity.SelectTpl2("辅助")
			fmt.Println(res)
			wg.Done()
		}(i)

	}
	wg.Wait()
}
//查询
func TestSelectAll(t *testing.T) {
	result := activity.SelectAll()
	fmt.Println("result=", result)
}

//非事务方式更新
func TestUpdate(t *testing.T) {
	activityBean := BizActivity{
		Id:   "9",
		Name: "风暴龙王",
		CreateTime: time.Now(),
		DeleteFlag: 1,
	}
	updateNum := activity.UpdateById(activityBean)
	fmt.Println("updateNum=", updateNum)
}

//软删除 设置 DeleteFlag = 0
func TestRemove(t *testing.T) {
	result := activity.RemoveById("3")
	fmt.Println("result=", result)
}

// 物理删除
func TestDelete(t *testing.T){
	for i:=0;i<20;i++ {
		result := activity.DeleteById(i)
		fmt.Println("result=", result)

	}
}

//物理删除(使用模板标签) 模板默认支持逻辑删除
func TestDeleteTemplate(t *testing.T) {

	result := activity.DeleteTemplate("刺客")
	fmt.Println("result=", result)

}

func TestSelectById(t *testing.T){
	res := activity.SelectById("5")
	fmt.Println(res)
}

//查询
func TestCount(t *testing.T) {
	start := time.Now().Add(-13*time.Hour)
	end := time.Now().Add(3*time.Hour)

	result := activity.CountByCondition("战士", start, end)
	fmt.Println("result=", result)
}

//循环
func TestForeach(t *testing.T) {

	ids := []string{"7", "8"}
	result := activity.SelectByIds(ids)
	fmt.Println("result=", result)
	fmt.Println("num=", len(result))

}

//循环
func TestForeachMap(t *testing.T) {

	ids := map[int]string{1: "4", 2: "5"}
	result := activity.SelectByIdMaps(ids)
	fmt.Println("result=", result)
	fmt.Println("num=", len(result))

}

func TestChoose(t *testing.T) {
	result := activity.Choose(1)
	for _,item := range result {
		fmt.Println("row = ",item)
	}
}

//查询
func TestIncludeSql(t *testing.T) {
	result := activity.SelectLinks("name")
	if len(result) >=0 {
		fmt.Println("result=", result)
		fmt.Println("num=", len(result))
	}
}

func TestSelectCountTemplate(t *testing.T) {
	result := activity.SelectCountTemplate("射手")
	fmt.Println("result=", result)
}

// 如果数据库中存在该记录则该测试函数会报错。
func TestInsertTemplate(t *testing.T) {
	activity.DeleteById(180)

	// 如果数据库中存在该记录则会报错。
	result := activity.InsertTemplate(BizActivity{Id: "180", Name: "刺客", PcLink:"露娜",CreateTime: time.Now(), Sort: 1, Status: 1, DeleteFlag: 1})
	fmt.Println("result=", result)

}

//批量插入模板，如果数据库中存在该记录则该测试函数会报错。
func TestInsertTemplateBatch(t *testing.T) {
	activity.DeleteByIds([]int{221, 222, 223})


	// 如果数据库中存在该记录则该测试函数会报错。
	args := []BizActivity{
		{
			Id:         "221",
			Name:       "辅助",
			PcLink:     "孙膑",
			CreateTime: time.Now(),
		},
		{
			Id:         "222",
			Name:       "辅助",
			PcLink:     "牛魔王",
			CreateTime: time.Now(),
		},
		{
			Id:         "223",
			Name:       "辅助",
			PcLink: 	"沈梦溪",
			CreateTime: time.Now(),
		},
	}
	n := activity.InsertTemplateBatch(args)
	fmt.Println("updateNum", n)

	//time.Sleep(time.Second)
}

// 修改模板默认支持逻辑删除和乐观锁
func TestUpdateTemplate(t *testing.T) {
	activityBean := BizActivity{
		Id:       "7",
		Name:   "法师",
		PcLink: "张良",
		Version: 0, //这个版本号是表中字段已经存在的版本号，它是作为where的条件。更新过后 Version会在原来的基础上自动+1。
	}
	//会自动生成乐观锁和逻辑删除字段 set version= * where version = * and delete_flag = *
	// update set name = '法师',version = 1 from biz_activity where delete_flag = 1 and version = 0 and id = 171
	updateNum := activity.UpdateTemplate(activityBean)
	fmt.Println("updateNum=", updateNum)

}

//使用事务
func TestLocalTransaction(t *testing.T) {
	en.Begin()
	bean1 := BizActivity{
		Id:   "7",
		Name: "刺客",
		PcLink: "百里玄策",
		DeleteFlag: 1,
		CreateTime: time.Now(),
	}
	updateNum := activity.UpdateById(bean1)
	fmt.Println("updateNum=", updateNum)
	en.Commit()
	en.Begin()
	//a2 := BizActivity{
	//	Id:   "15",
	//	Name: "战士",
	//	PcLink: "云樱",
	//	DeleteFlag: 1,
	//	CreateTime: time.Now(),
	//}
	//insert := activity.Insert2(a2)
	//fmt.Println("insertNum = ",insert)
	en.Commit()
	bean2 := BizActivity{
		Id:   "4",
		Name: "刺客",
		PcLink: "百里玄策",
		DeleteFlag: 1,
		CreateTime: time.Now(),
	}
	updateNum2 := activity.UpdateById(bean2)
	fmt.Println("updateNum=", updateNum2)
}

// 操作 MysqlUri1 对应的 test1 数据库
// 注意: test1 数据库中只有 ums_admin ums_role 这两张表
func init(){
	//第一步: 创建一个 engine对象,一个engine对象操作一个SQL数据库。
	//eng := mbt.New(cfg.Database2)

	//第二步: 创建映射关系
	//h := mbt.H{
	//	&admin: &UmsAdmin{},
	//	&role:  &UmsRole{},
	//}

	//第三步: 将映射关系注册到engine对象中
	//eng.Register(h)

	//第四步: 启动服务(如果只单纯测试项目中的SQL语句是否正确,则这一步可以先注释掉)
	//eng.Run(&admin,&role)
}

var db *sqlx.DB

func init(){
	conn,_ := sqlx.Connect(cfg.Database2.DriverName, cfg.Database2.DSN)
	db = conn
}

// 参数与返回值都不要定义指针类型的数据库表实体。
type AdminDao struct {
	UmsAdmin UmsAdmin
	InsertTemplate     func(arg []UmsAdmin) int64
	UsernameEqualTo    func(username string) []UmsAdmin `arg:"username"`
	Insert             func(arg UmsAdmin) int64
	SelectByPrimaryKey func(id int64) UmsAdmin                  `arg:"id"`
	UserExist          func(username, password string) UmsAdmin `arg:"username,password"`
	UpdateColumn       func(loginTime time.Time) int64          `arg:"loginTime"`

	UpdateByPrimaryKeySelective func(args UmsAdmin) int64
	UpdatePassword              func(password string, id int64) int64    `arg:"password,id"`
	GetAdmin                    func(username, password string) UmsAdmin `arg:"username,password"`
	DeleteByPrimaryKey          func(id int) int64                       `arg:"id"` // 物理删除一条记录
	GetBase                     func() []BaseInfo
	ListWithChildren            func() []UmsAdmin
	DeleteByIds                 func(ids []int) int64 `arg:"ids"`
	SelectUnion                 func() []UmsAdmin2
	//SelectUnion                 func() UmsAdmin3
	CreateUmsRole  func() int
	CreateUmsAdmin func() int
	CreateBizActivity func()int

	GetDB func ()[]Num
}

type Num struct {
	O int `json:"tablename"`
}

func TestGetDB(t *testing.T){
	 num := admin.GetDB()
	 fmt.Print(num)
}

// 往 test1数据库ums_admin表中批量插入记录
func TestUmsAdminInsertTemplate(t *testing.T){
	u1 := UmsAdmin{
		Id: 1,
		ParentId: 0,
		Username: "打野",
		Password: "123456",
		Icon: "天美",
		Email: "123@qq.com",
		NickName: "演员",
		Note: "从头打到尾",
		CreateTime: time.Now(),
		LoginTime: time.Now(),
		Status: 1,
	}

	u2 := UmsAdmin{
		Id: 2,
		ParentId: 1,
		Username: "赵云",
		Password: "123456",
		Icon: "天美",
		Email: "123@qq.com",
		NickName: "战士",
		Note: "半肉半输出",
		CreateTime: time.Now(),
		LoginTime: time.Now(),
		Status: 0,
	}

	u3 := UmsAdmin{
		Id: 3,
		ParentId: 1,
		Username: "澜",
		Password: "123456",
		Icon: "天美",
		Email: "123@qq.com",
		NickName: "刺客",
		Note: "全输出",
		CreateTime: time.Now(),
		LoginTime: time.Now(),
		Status: 0,
	}

	u4 := UmsAdmin{
		Id: 4,
		ParentId: 1,
		Username: "宫本武藏",
		Password: "123456",
		Icon: "天美",
		Email: "123@qq.com",
		NickName: "超级兵",
		Note: "全肉",
		CreateTime: time.Now(),
		LoginTime: time.Now(),
		Status: 0,
	}

	u5 := UmsAdmin{
		Id: 5,
		ParentId: 1,
		Username: "李白",
		Password: "123456",
		Icon: "天美",
		Email: "123@qq.com",
		NickName: "刮痧李师傅",
		Note: "全输出",
		CreateTime: time.Now(),
		LoginTime: time.Now(),
		Status: 0,
	}

	u6 := UmsAdmin{
		Id: 6,
		ParentId: 2,
		Username: "安琪拉",
		Password: "66666",
		Icon: "天美",
		Email: "123@qq.com",
		NickName: "姐妹",
		Note: "法师",
		CreateTime: time.Now(),
		LoginTime: time.Now(),
		Status: 0,
	}

	u7 := UmsAdmin{
		Id: 7,
		ParentId: 2,
		Username: "芈月",
		Password: "8888",
		Icon: "天美",
		Email: "123@qq.com",
		NickName: "吸血",
		Note: "法师",
		CreateTime: time.Now(),
		LoginTime: time.Now(),
		Status: 0,
	}

	u8 := UmsAdmin{
		Id: 8,
		ParentId: 2,
		Username: "貂蝉",
		Password: "891235",
		Icon: "天美",
		Email: "123@qq.com",
		NickName: "我的貂蝉在哪里",
		Note: "法师",
		CreateTime: time.Now(),
		LoginTime: time.Now(),
		Status: 0,
	}
	list := make([]UmsAdmin,0)
	list = append(list,u1,u2,u3,u4,u5,u6,u7,u8)
	res := admin.InsertTemplate(list)
	fmt.Println(res)
}

func TestPostgres(t *testing.T){
	fmt.Println(admin.CreateUmsRole())
	//fmt.Println(admin.CreateUmsAdmin())
	//fmt.Println(admin.CreateBizActivity())
}


func TestAdminDaoFind(t *testing.T){
	fmt.Println(admin.GetAdmin("打野","123456"))
}

func TestAdminDaoDelete(t *testing.T){
	ids := admin.DeleteByIds([]int{1,2,3,4,5,6,7,8})
	fmt.Println(ids)
}

func TestAdminDao(t *testing.T){
	all := admin.UsernameEqualTo("澜")
	fmt.Println(all)
}

func TestAdminDaoChildren(t *testing.T){
	u := admin.ListWithChildren()
	fmt.Println(u[0])
	fmt.Println(u[1])
	fmt.Println(u[2])
	fmt.Println(u[3])
	fmt.Println("num =",len(u))
}

// 使用 sqlx 内联关联查询
func TestAdminSqlx(t *testing.T){
	var obj []UmsAdmin2
	queryStr := `select u.*,
		       r.id r_id, r.pid, r.name, r.description, r.admin_count, r.create_time r_create_time, r.status r_status, r.sort
		from ums_admin as u
				 left join ums_role as r on r.pid = u.id`
	db.Select(&obj,queryStr)
	fmt.Println(obj)
	fmt.Println(len(obj))
}

// 使用 mbt 内联关联查询
func TestAdminDaoSelectUnion(t *testing.T){
	var obj UmsAdmin3
	union := admin.SelectUnion()
	for _,item := range union  {
		ums := UmsAdmin{
			Id :item.Id  ,
			ParentId :item.ParentId,
			Username :item.Username,
			Password :item.Password,
			Icon     :item.Icon,
			Email    :item.Email,
			NickName :item.NickName,
			Note     :item.Note,
			CreateTime :item.CreateTime,
			LoginTime  :item.LoginTime,
			Status     :item.Status,
		}
		
		obj.UmsAdmin = append(obj.UmsAdmin,ums)
		rol := UmsRole2{
			RId : item.RId,
			Pid :item.Pid,
			Name : item.Name,
			Description : item.Description,
			AdminCount: item.AdminCount,
			RCreateTime: item.RCreateTime,
			RStatus: item.RStatus,
			Sort: item.Sort,
		}
		obj.UmsRole2 = append(obj.UmsRole2,rol)
	}
	fmt.Println(removeRepByMap(obj.UmsAdmin))
}
//slice去重
func removeRepByMap(slc []UmsAdmin) []UmsAdmin {
	result := make([]UmsAdmin,0)         //存放返回的不重复切片
	tempMap := map[interface{}]byte{} // 存放不重复主键
	for _, e := range slc {
		l := len(tempMap)
		tempMap[e] = 0 //当e存在于tempMap中时，再次添加是添加不进去的，，因为key不允许重复
		//如果上一行添加成功，那么长度发生变化且此时元素一定不重复
		if len(tempMap) != l { // 加入map后，map长度变化，则元素不重复
			result = append(result, e) //当元素不重复时，将元素添加到切片result中
		}
	}
	return result
}

type RoleDao struct {
	UmsRole UmsRole
	Insert    func(args UmsRole) int64
	GetItem   func(id int64) UmsRole `arg:"id"`
	DeleteAll func() int
}

func TestRoleDaoInsert(t *testing.T){
	role.Insert(UmsRole{Id: 1,Name: "射手",Pid: 1,Description: "虞姬,后裔,鲁班,狄仁杰",AdminCount: 4,CreateTime: time.Now(),Status: 1,Sort: 1})
	role.Insert(UmsRole{Id: 2,Name: "刺客",Pid: 1,Description: "孙悟空,兰陵王,阿轲,澜",AdminCount: 3,CreateTime: time.Now(),Status: 1,Sort: 1})
	role.Insert(UmsRole{Id: 3,Name: "法师",Pid: 1,Description: "妲己,安琪拉,甄姬,貂蝉",AdminCount: 2,CreateTime: time.Now(),Status: 1,Sort: 1})

	role.Insert(UmsRole{Id: 4,Name: "辅助",Pid: 2,Description: "太乙真人,廉颇,张飞,庄周",AdminCount: 5,CreateTime: time.Now(),Status: 1,Sort: 1})
	role.Insert(UmsRole{Id: 5,Name: "游走",Pid: 2,Description: "孙膑,蔡文姬,沈梦溪",AdminCount: 6,CreateTime: time.Now(),Status: 1,Sort: 1})
	role.Insert(UmsRole{Id: 6,Name: "战士",Pid: 2,Description: "凯,典韦,曹操,吕布",AdminCount: 7,CreateTime: time.Now(),Status: 1,Sort: 1})
}

func TestDeleteAll(t *testing.T){
	fmt.Println(role.DeleteAll())
}

var cst *time.Location
const CSTLayout = "2006-01-02 15:04:05"

func init(){
	var err error
	if cst, err = time.LoadLocation("Asia/Shanghai"); err != nil {
		panic(err)
	}
}

func RFC3339ToCSTLayout(value string)string {
	ts, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		panic(err)
	}
	return ts.Format(CSTLayout)
	//return ts.In(cst).Format(CSTLayout)
}

func TestRoleDao(t *testing.T){
	item := role.GetItem(1)
	//fmt.Println(RFC3339ToCSTLayout(item.CreateTime.String()))


	//fmt.Println(item.CreateTime)

	fmt.Println(Format(item.CreateTime.String()))
}

// 切割时间类型的字符串
func Format(str string)string{
	split := strings.Split(str, ".")
	return split[0]
}

func TestRoleTimeFormat(t *testing.T){
	timeStr := "2021-07-17 15:54:29 +0000 UTC"
	layout := RFC3339ToCSTLayout(timeStr)
	fmt.Println(layout)
}

func getDomain(strS string){
	//contente,err := ioutil.ReadFile("./input_data.txt")
	//if err != nil {
	//	panic(err)
	//}
	//strS := string(contente)

	no1 := strings.Split(strS, ":")
	no2 := no1[3]
	no3 := strings.Split(no2, " ")
	no4 := no3[2]
	fmt.Println(no4)
}



func TestReadAll(t *testing.T){

	//if err != nil {
	//	panic(err)
	//}
	//str := string(contente)
	//
	//no1 := strings.Split(str, ":")
	//no2 := no1[3]
	//no3 := strings.Split(no2, " ")
	//no4 := no3[2]
	//fmt.Println(no4)

	//file,err := ioutil.ReadFile("./input_data.txt")
	file,err := os.Open("input_data.txt")
	if err != nil {
		panic(err)
	}
	defer file.Close()
	reader := bufio.NewReader(file)

	for {
		str,_ := reader.ReadString('\n')
		getDomain(str)
	}





}
