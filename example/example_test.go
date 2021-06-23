package example

import (
	"encoding/json"
	"fmt"
	"github.com/fsnotify/fsnotify"
	_ "github.com/go-sql-driver/mysql"
	"github.com/kotlin2018/mbt"
	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	_ "github.com/lib/pq"
	"github.com/spf13/viper"
	"io"
	"log"
	"testing"
	"time"
)

var (
	en       *mbt.Engine
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
	engine,_,err := mbt.New(cfg.Database)
	en = engine
	if err != nil {
		log.Fatalln(err)
	}

	//第二步: 创建映射关系
	// BizActivity{} 即:数据库中表biz_activity的实体类，这层映射表示: activity这个结构体对象对表 biz_activity 进行 CRUD操作!
	h := mbt.H{
		&activity: &BizActivity{},
		//&activity: "BizActivity",
	}

	//第三步: 将映射关系注册到engine对象中
	engine.Register(h)

	// 可选,生产环境下建议使用此功能
	engine.SetOutPut(initLogger(cfg.Database.Logger.Path, cfg.Database.Logger.MaxAge, cfg.Database.Logger.Interval))

	//第四步: 启动服务(如果只单纯测试项目中的SQL语句是否正确,则这一步可以先注释掉)
	engine.Run()
}

// 操作 MysqlUri1 对应的 test1 数据库
// 注意: test1 数据库中只有 ums_admin ums_role 这两张表
func init(){
	//第一步: 创建一个 engine对象,一个engine对象操作一个SQL数据库。
	engine,_,err := mbt.New(cfg.Database2)
	if err != nil {
		panic(err)
	}

	//第二步: 创建映射关系
	h := mbt.H{
		&admin: &UmsAdmin{},
		//&role:  &UmsRole{},
		&role: "UmsRole",
	}

	//第三步: 将映射关系注册到engine对象中
	engine.Register(h)

	//第四步: 启动服务(如果只单纯测试项目中的SQL语句是否正确,则这一步可以先注释掉)
	engine.Run()
}

// 对数据库表所有的CRUD操作都在这个结构体里面
// 原则上来说 一个 xxxDao结构体只操作一张表,实际您也可以 一个 xxxDao 结构体操作多张表。
type ActivityDao struct {
	Tx mbt.Tx //session事务操作
	InsertTemplate      func(arg Activity) (int64, error)
	InsertTemplateBatch func(args []Activity) (int64, error)
	UpdateTemplate      func(arg Activity) (int64, error)
	SelectTemplate      func(arg Activity) ([]Activity, error)
	SelectTpl           func(name string) (Activity, error)   `arg:"name"`
	SelectCountTemplate func(name string) (int64, error)      `arg:"name"`
	DeleteTemplate      func(name string) (int64, error)      `arg:"name"`

	SelectByIds    func(ids []string) ([]Activity, error)       `arg:"ids"`
	SelectByIds2   func(ids []string) ([]Activity, error)       `arg:"ids"`
	SelectByIdMaps func(ids map[int]string) ([]Activity, error) `arg:"ids"`
	SelectAll      func(session mbt.Session) ([]map[string]string, error)
	SelectById     func(id string)(Activity,error)`arg:"id"`

	SelectByCondition  func(base Base) ([]BizActivity, error)
	SelectByCondition2 func(base Base) ([]BizActivity, error)
	SelectByName       func(name, pcLink string) (Activity, error)`arg:"name,pc_link"`
	UpdateById         func(arg Activity,session mbt.Session) (int64, error)
	Insert             func(arg Activity) (int64, error)
	CountByCondition   func(name string, startTime time.Time, endTime time.Time) (int, error) `arg:"name,start_time,end_time"`
	RemoveById         func(id string) (int64, error)                                         `arg:"id"`
	DeleteById         func(id int) (int64, error)                                            `arg:"id"`
	DeleteByIds        func(ids []int)(int64,error)`arg:"ids"`

	Choose             func(deleteFlag int) ([]Activity, error)                               `arg:"delete_flag"`
	Choose2            func(arg Choose)([]Activity,error)
	SelectLinks        func(column string) ([]Activity, error)                                `arg:"column"`
}

func TestSelectByName(t *testing.T) {
	res, err := activity.SelectByName("刺客", "韩信")
	if err == nil && res.Id != "" {
		fmt.Println(res)
	}
}

//插入
func TestInset(t *testing.T) {
	activity.Insert(Activity{Id: "1", Name: "刺客", PcLink: "韩信", CreateTime: time.Now(), DeleteFlag: 1})
	activity.Insert(Activity{Id: "2", Name: "刺客", PcLink: "孙悟空", CreateTime: time.Now(), DeleteFlag: 1})
	activity.Insert(Activity{Id: "3", Name: "刺客", PcLink: "兰陵王", CreateTime: time.Now(), DeleteFlag: 1})
	activity.Insert(Activity{Id: "4", Name: "法师", PcLink: "妲己", CreateTime: time.Now(), DeleteFlag: 1})
	activity.Insert(Activity{Id: "5", Name: "法师", PcLink: "安琪拉", CreateTime: time.Now(), DeleteFlag: 1})
	activity.Insert(Activity{Id: "6", Name: "战士", PcLink: "夏侯敦", CreateTime: time.Now(), DeleteFlag: 1})
	activity.Insert(Activity{Id: "7", Name: "战士", PcLink: "凯", CreateTime: time.Now(), DeleteFlag: 1})
	activity.Insert(Activity{Id: "8", Name: "射手", PcLink: "鲁班", CreateTime: time.Now(), DeleteFlag: 1})
	activity.Insert(Activity{Id: "9", Name: "射手", PcLink: "虞姬", CreateTime: time.Now(), DeleteFlag: 1})
	activity.Insert(Activity{Id: "10", Name: "射手", PcLink: "后裔", CreateTime: time.Now(), DeleteFlag: 1})
	activity.Insert(Activity{Id: "11", Name: "辅助", PcLink: "太乙真人", CreateTime: time.Now(), DeleteFlag: 1})
	//role.Insert(UmsRole{Name: "射手",Description: "虞姬,后裔,鲁班,狄仁杰",AdminCount: 4,CreateTime: time.Now(),Status: 1,Sort: 1})
}

// 通过传入不同的参数来复用同一个方法,sql语句中 where 条件之间用 or来连接!!!
func TestChoose2(t *testing.T){
	c1 := Choose{
		Name: "刺客",
	}
	res, err := activity.Choose2(c1)
	if err == nil {
		fmt.Println(len(res))
	}
	c2 := Choose{
		PcLink: "后羿",
	}
	res2, err2 := activity.Choose2(c2)
	if err2 == nil {
		fmt.Println(len(res2))
	}
}

//查询 通过传入不同的参数来复用同一个方法,sql语句中 where 条件之间用 or来连接!!!
// 注释掉:biz_activity.xml文件中 <resultMap>标签中的 <result column="delete_flag">这一项,则 delete_flag = 0的记录也会被查询出来!
func TestSelectTemplate(t *testing.T) {
	ac1 := Activity{
		//Name: "刺客",
		PcLink: "后羿",
	}
	result, err := activity.SelectTemplate(ac1)
	if err == nil {
		fmt.Println("result=", result)
	}
}

//非事务方式更新
func TestUpdate(t *testing.T) {
	activityBean := Activity{
		Id:   "11",
		Name: "风暴龙王",
		CreateTime: time.Now(),
		DeleteFlag: 1,
	}
	//sessionId 有值则使用已经创建的session，否则新建一个session
	updateNum, err := activity.UpdateById(activityBean,nil)
	fmt.Println("updateNum=", updateNum)
	if err != nil {
		panic(err)
	}
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
	list := make([]UmsAdmin,0)
	list = append(list,u1,u2,u3,u4,u5)
	res, err := admin.InsertTemplate(list)
	fmt.Println(err)
	fmt.Println(res)
}

//联合查询,分别从 test、test1数据库中查询出记录,此功能适用于分库分表。
func TestSelect(t *testing.T) {
	base := Base{
		Name: "刺客",
		Page: 0,
		Size: 2,
		StartTime: time.Now().Add(-9*time.Hour),
		EndTime: time.Now().Add(2*time.Hour),
	}

	result, err := activity.SelectByCondition2(base)

	if err != nil {
		panic(err)
	}

	for _,item := range result {
		fmt.Println("result=", item)
	}
	fmt.Println("num=", len(result))

	u, err := admin.ListWithChildren()
	if err !=nil {
		panic(err)
	}
	fmt.Println(u[0])
	fmt.Println(u[1])
	fmt.Println(u[2])
	fmt.Println(u[3])
	fmt.Println("num =",len(u))

	//item, err := role.GetItem(1)
	//if err !=nil {
	//	panic(err)
	//}
	//fmt.Println(item)
}

//软删除 设置 DeleteFlag = 0
func TestRemove(t *testing.T) {
	result, err := activity.RemoveById("11")
	if err != nil {
		panic(err)
	}
	fmt.Println("result=", result)
}

// 物理删除
func TestDelete(t *testing.T){
	for i:=0;i<20;i++ {
		result, err := activity.DeleteById(i)
		if err != nil {
			panic(err)
		}
		fmt.Println("result=", result)
	}
}

//物理删除(使用模板标签) 模板默认支持逻辑删除
func TestDeleteTemplate(t *testing.T) {

	result, err := activity.DeleteTemplate("刺客")
	if err != nil {
		panic(err)
	}
	fmt.Println("result=", result)
}

//查询
func TestSelectAll(t *testing.T) {
	//session := activity.Tx()
	//session.Begin()
	result, err := activity.SelectAll(nil)
	if err == nil {
		b, _ := json.Marshal(result)
		fmt.Println("result=", string(b))
		fmt.Println("num=", len(result))
	}
}

func TestSelectById(t *testing.T){
	res, err := activity.SelectById("1")
	if err == nil {
		fmt.Println(res)
	}
}

//查询
func TestCount(t *testing.T) {
	start := time.Now().Add(-1*time.Hour)
	end := time.Now().Add(1*time.Hour)

	result, err := activity.CountByCondition("战士", start, end)
	if err != nil {
		panic(err)
	}
	fmt.Println("result=", result)
}

//循环
func TestForeach(t *testing.T) {

	ids := []string{"1", "2"}
	result, err := activity.SelectByIds(ids)

	if err != nil {
		panic(err)
	}
	fmt.Println("result=", result)
	fmt.Println("num=", len(result))
}

//循环
func TestForeachMap(t *testing.T) {

	ids := map[int]string{1: "4", 2: "5"}
	result, err := activity.SelectByIdMaps(ids)

	if err != nil {
		panic(err)
	}
	fmt.Println("result=", result)
	fmt.Println("num=", len(result))
}

func TestChoose(t *testing.T) {
	result, err := activity.Choose(0)
	if err != nil {
		panic(err)
	}
	fmt.Println("result=", len(result))
	fmt.Println("result=", result)
}

//查询
func TestIncludeSql(t *testing.T) {

	result, err := activity.SelectLinks("name")
	if err != nil {
		panic(err)
	}
	fmt.Println("result=", result)
	fmt.Println("num=", len(result))
}

func TestSelectCountTemplate(t *testing.T) {

	result, err := activity.SelectCountTemplate("射手")
	if err != nil {
		panic(err)
	}
	fmt.Println("result=", result)
}

// 如果数据库中存在该记录则该测试函数会报错。
func TestInsertTemplate(t *testing.T) {
	_, err := activity.DeleteById(180)
	if err != nil {
		panic(err)
	}
	// 如果数据库中存在该记录则会报错。
	result, err := activity.InsertTemplate(Activity{Id: "180", Name: "刺客", PcLink:"露娜",CreateTime: time.Now(), Sort: 1, Status: 1, DeleteFlag: 1})
	if err != nil {
		panic(err)
	}
	fmt.Println("result=", result)
}

//批量插入模板，如果数据库中存在该记录则该测试函数会报错。
func TestInsertTemplateBatch(t *testing.T) {
	_, err := activity.DeleteByIds([]int{221, 222, 223})
	if err !=nil {
		panic(err)
	}

	// 如果数据库中存在该记录则该测试函数会报错。
	args := []Activity{
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
	n, err := activity.InsertTemplateBatch(args)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("updateNum", n)
	//time.Sleep(time.Second)
}

//修改模板默认支持逻辑删除和乐观锁
func TestUpdateTemplate(t *testing.T) {
	activityBean := Activity{
		Id:       "1",
		Name:   "法师",
		PcLink: "张良",
		Version: 0, //这个版本号是表中字段已经存在的版本号，它是作为where的条件。更新过后 Version会在原来的基础上自动+1。
	}
	//会自动生成乐观锁和逻辑删除字段 set version= * where version = * and delete_flag = *
	// update set name = '法师',version = 1 from biz_activity where delete_flag = 1 and version = 0 and id = 171
	var updateNum, e = activity.UpdateTemplate(activityBean)
	fmt.Println("updateNum=", updateNum)
	if e != nil {
		panic(e)
	}
}

//第一种使用事务的方法: 本地事务
func TestLocalTransaction(t *testing.T) {
	//使用事务
	session := activity.Tx()
	//使用本地事务
	session.Begin()
	activityBean := Activity{
		Id:   "2",
		Name: "刺客",
		PcLink: "百里玄策",
		H5Link: "边路",
		Remark: "不要与猪八戒刚",
		DeleteFlag: 1,
		CreateTime: time.Now(),

	}
	all, err := activity.SelectAll(session) //查询事务应该放在执行事务之前
	if err == nil {
		fmt.Println(all)
	}
	//sessionId 有值则使用已经创建的session，否则新建一个session
	updateNum, err := activity.UpdateById(activityBean,session)
	if err == nil {
		session.Commit()
	}else {
		session.Rollback()
	}
	fmt.Println("updateNum=", updateNum)
	if err != nil {
		panic(err)
	}
}

//第二种使用事务的方法: 代理试事务
type TestService struct {
	it *ActivityDao //服务包含一个mapper操作数据库
	UpdateRemark func(id string, name string) error
	UpdateName   func(id string, name string) error
}

//第二种使用事务的方法: 嵌套事务/带有传播行为的事务
func TestTestService(t *testing.T) {
	testService := initTestService()
	testService.UpdateName("6", "法师")
	testService.UpdateRemark("7","射手")
}

func initTestService() TestService {
	var testService TestService
	testService = TestService{
		it: &activity,
		UpdateRemark: func(id string, name string) error {
			a := Activity{
				Id:   id,
				Name: name,
				PcLink: "肉坦",
				H5Link: "边路",
				DeleteFlag: 1,
				CreateTime: time.Now(),
			}
			updateNum, err := testService.it.UpdateById(a,nil)
			if err == nil {
				fmt.Println("updateNum = ",updateNum)
			}
			all, err := testService.it.SelectAll(nil)
			if err != nil {
				fmt.Println(all)
			}
			return err
		},

		UpdateName: func(id string, name string) error {
			a := Activity{
				Id: id,
				Name: name,
				CreateTime: time.Now(),
			}
			updateNum, err := testService.it.UpdateTemplate(a)
			if err == nil {
				fmt.Println(updateNum)
			}
			all, err := testService.it.SelectAll(nil)
			if err != nil {
				fmt.Println(all)
			}
			return nil

		},
	}
	en.Tx(&testService)
	return testService
}

// 参数与返回值都不要定义指针类型的数据库表实体。
type AdminDao struct {
	// 如果有返回值，则说明value已经注册过了
	UsernameEqualTo             func(username string) ([]UmsAdmin, error) `arg:"username"`
	Insert                      func(arg UmsAdmin) (int64, error)
	SelectByPrimaryKey          func(id int64) (UmsAdmin, error)                  `arg:"id"`
	UserExist                   func(username, password string) (UmsAdmin, error) `arg:"username,password"`
	UpdateColumn                func(loginTime time.Time) (int64, error)          `arg:"loginTime"`
	InsertTemplate              func(arg []UmsAdmin) (int64, error)
	UpdateByPrimaryKeySelective func(args UmsAdmin) (int64, error)
	UpdatePassword              func(password string, id int64) (int64, error)    `arg:"password,id"`
	GetAdmin                    func(username, password string) (UmsAdmin, error) `arg:"username,password"`
	DeleteByPrimaryKey          func(id int) (int64, error)                       `arg:"id"` // 物理删除一条记录
	GetBase                     func() ([]BaseInfo, error)
	ListWithChildren            func() ([]UmsAdmin, error)
}

func TestAdminDao(t *testing.T){
	all, err := admin.UsernameEqualTo("澜")
	if err == nil {
		fmt.Println(all)
	}
}

type RoleDao struct {
	Insert func(args UmsRole) (int64, error)
	GetItem func(id int64)(UmsRole,error) `arg:"id"`
}

func TestRoleDao(t *testing.T){
	item, err := role.GetItem(1)
	if err !=nil {
		panic(err)
	}
	fmt.Println(item)
}